package config

import (
	"context"
	"log"
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"dario.cat/mergo"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xreflect"
	"github.com/jinzhu/copier"
)

// Configurator manages a set of config sources and exposes the config loaded from them.
type Configurator interface {
	// Has reports whether a config matching pattern exists.
	Has(pattern string) bool
	// Get returns the config value under pattern, falling back to def when the config is missing.
	Get(pattern string, def ...any) value.Value
	// Set sets the config value under pattern.
	Set(pattern string, value any) error
	// Match returns a [Matcher] over the given patterns.
	Match(patterns ...string) Matcher
	// Watch registers cb to be called when one of the named configs changes; empty names means every
	// config.
	Watch(cb WatchCallbackFunc, names ...string)
	// Load loads the configurations of the named source; empty file means every file of the source.
	Load(ctx context.Context, source string, file ...string) ([]*Configuration, error)
	// Store saves content as file under the named source, merging it with the existing config unless
	// override is true.
	Store(ctx context.Context, source string, file string, content any, override ...bool) error
	// Close closes the configurator and its config sources.
	Close()
}

// WatchCallbackFunc is called with the names of the configs that changed.
type WatchCallbackFunc func(names ...string)

type watcher struct {
	names    map[string]struct{}
	callback WatchCallbackFunc
}

type defaultConfigurator struct {
	opts     *options
	ctx      context.Context
	cancel   context.CancelFunc
	sources  map[string]Source
	mu       sync.Mutex
	idx      atomic.Int64
	values   [2]map[string]any
	rw       sync.RWMutex
	watchers []*watcher
}

var _ Configurator = &defaultConfigurator{}

// NewConfigurator returns a new [Configurator] configured with opts and starts watching the config
// sources for changes.
func NewConfigurator(opts ...Option) Configurator {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	r := &defaultConfigurator{}
	r.opts = o
	r.ctx, r.cancel = context.WithCancel(o.ctx)
	r.watchers = make([]*watcher, 0)
	r.init()
	r.watch()

	return r
}

// init initializes the config sources and loads their config into the value store.
func (c *defaultConfigurator) init() {
	c.sources = make(map[string]Source, len(c.opts.sources))
	for _, s := range c.opts.sources {
		c.sources[s.Name()] = s
	}

	values := make(map[string]any)
	for _, s := range c.opts.sources {
		cs, err := s.Load(c.ctx)
		if err != nil {
			log.Printf("load configure failed: %v", err)
			continue
		}

		for _, cc := range cs {
			if len(cc.Content) == 0 {
				continue
			}

			v, err := c.opts.decoder(cc.Format, cc.Content)
			if err != nil {
				if !errors.Is(err, errors.ErrInvalidFormat) {
					log.Printf("decode configure failed: %v", err)
				}
				continue
			}

			values[cc.Name] = v
		}
	}

	c.store(values)
}

// store publishes values to the slot that follows the current index.
func (c *defaultConfigurator) store(values map[string]any) {
	c.values[c.idx.Add(1)%int64(len(c.values))] = values
}

// load returns the values of the slot that follows the current index.
func (c *defaultConfigurator) load() map[string]any {
	return c.values[c.idx.Load()%int64(len(c.values))]
}

// copy returns a deep copy of the currently loaded values.
func (c *defaultConfigurator) copy() (map[string]any, error) {
	values := c.load()

	dst := make(map[string]any)

	err := copier.CopyWithOption(&dst, values, copier.Option{
		DeepCopy: true,
	})
	if err != nil {
		return nil, err
	}

	return dst, nil
}

// watch starts a goroutine per source that watches it and merges its changes into the store.
func (c *defaultConfigurator) watch() {
	for _, s := range c.opts.sources {
		w, err := s.Watch(c.ctx)
		if err != nil {
			log.Printf("watching configure change failed: %v", err)
			continue
		}

		go func() {
			defer w.Stop()

			for {
				select {
				case <-c.ctx.Done():
					return
				default:
					// exec watch
				}
				cs, err := w.Next()
				if err != nil {
					continue
				}

				names := make([]string, 0, len(cs))
				values := make(map[string]any)
				for _, cc := range cs {
					if len(cc.Content) == 0 {
						continue
					}

					v, err := c.opts.decoder(cc.Format, cc.Content)
					if err != nil {
						continue
					}
					names = append(names, cc.Name)
					values[cc.Name] = v
				}

				func() {
					c.mu.Lock()
					defer c.mu.Unlock()

					dst, err := c.copy()
					if err != nil {
						return
					}

					err = mergo.Merge(&dst, values, mergo.WithOverride)
					if err != nil {
						return
					}

					c.store(dst)
				}()

				if len(names) > 0 {
					go c.notify(names...)
				}
			}
		}()
	}
}

// notify calls every watcher that matches one of the given names.
func (c *defaultConfigurator) notify(names ...string) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	for _, w := range c.watchers {
		if len(w.names) == 0 {
			w.callback(names...)
		} else {
			validNames := make([]string, 0, int(math.Min(float64(len(w.names)), float64(len(names)))))
			for _, name := range names {
				if _, ok := w.names[name]; ok {
					validNames = append(validNames, name)
				}
			}

			if len(validNames) > 0 {
				w.callback(validNames...)
			}
		}
	}
}

// Close closes the configurator and its config sources.
func (c *defaultConfigurator) Close() {
	c.cancel()

	for _, source := range c.sources {
		_ = source.Close()
	}
}

// Has reports whether a config matching pattern exists.
func (c *defaultConfigurator) Has(pattern string) bool {
	return c.doHas(pattern)
}

// doHas reports whether pattern resolves to a value in the loaded config.
func (c *defaultConfigurator) doHas(pattern string) bool {
	var (
		keys   = strings.Split(pattern, ".")
		node   any
		found  = true
		values = c.load()
	)

	keys = reviseKeys(keys, values)
	node = values
	for _, key := range keys {
		switch vs := node.(type) {
		case map[string]any:
			if v, ok := vs[key]; ok {
				node = v
			} else {
				found = false
			}
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil {
				found = false
			} else if len(vs) > i {
				node = vs[i]
			} else {
				found = false
			}
		default:
			found = false
		}

		if !found {
			break
		}
	}

	return found
}

// Get returns the config value under pattern, falling back to def when the config is missing.
func (c *defaultConfigurator) Get(pattern string, def ...any) value.Value {
	if val, ok := c.doGet(pattern); ok {
		return val
	}

	return value.NewValue(def...)
}

// Match returns a [Matcher] over the given patterns.
func (c *defaultConfigurator) Match(patterns ...string) Matcher {
	return &defaultMatcher{c: c, patterns: patterns}
}

// doGet resolves pattern against the loaded values.
func (c *defaultConfigurator) doGet(pattern string) (value.Value, bool) {
	var (
		keys   = strings.Split(pattern, ".")
		node   any
		found  = true
		values = c.load()
	)

	if len(values) == 0 {
		goto NOTFOUND
	}

	keys = reviseKeys(keys, values)
	node = values
	for _, key := range keys {
		switch vs := node.(type) {
		case map[string]any:
			if v, ok := vs[key]; ok {
				node = v
			} else {
				found = false
			}
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil {
				found = false
			} else if len(vs) > i {
				node = vs[i]
			} else {
				found = false
			}
		default:
			found = false
		}

		if !found {
			break
		}
	}

	if found {
		return value.NewValue(node), true
	}

NOTFOUND:
	return nil, false
}

// Set sets the config value under pattern.
func (c *defaultConfigurator) Set(pattern string, value any) error {
	var (
		keys = strings.Split(pattern, ".")
		node any
	)

	c.mu.Lock()
	defer c.mu.Unlock()

	values, err := c.copy()
	if err != nil {
		return err
	}

	keys = reviseKeys(keys, values)
	node = values
	for i, key := range keys {
		switch vs := node.(type) {
		case map[string]any:
			if i == len(keys)-1 {
				vs[key] = value
			} else {
				rebuild := false
				ii, err := strconv.Atoi(keys[i+1])
				if next, ok := vs[key]; ok {
					switch nv := next.(type) {
					case map[string]any:
						rebuild = err == nil
					case []any:
						rebuild = err != nil
						// the next node capacity is not enough
						// expand capacity
						if err == nil && ii >= len(nv) {
							dst := make([]any, ii+1)
							copy(dst, nv)
							vs[key] = dst
						}
					default:
						rebuild = true
					}
				} else {
					rebuild = true
				}

				if rebuild {
					if err != nil {
						vs[key] = make(map[string]any)
					} else {
						vs[key] = make([]any, 1)
					}
				}

				node = vs[key]
			}
		case []any:
			ii, err := strconv.Atoi(key)
			if err != nil {
				return err
			}

			if ii >= len(vs) {
				return errors.New("index overflow")
			}

			if i == len(keys)-1 {
				vs[ii] = value
			} else {
				rebuild := false
				_, err = strconv.Atoi(keys[i+1])
				switch nv := vs[ii].(type) {
				case map[string]any:
					rebuild = err == nil
				case []any:
					rebuild = err != nil
					// the next node capacity is not enough
					// expand capacity
					if err == nil && ii >= len(nv) {
						dst := make([]any, ii+1)
						copy(dst, nv)
						vs[ii] = dst
					}
				default:
					rebuild = true
				}

				if rebuild {
					if err != nil {
						vs[ii] = make(map[string]any)
					} else {
						vs[ii] = make([]any, 1)
					}
				}

				node = vs[ii]
			}
		}
	}

	c.store(values)

	return nil
}

// Watch registers cb to be called when one of the named configs changes; empty names means every
// config.
func (c *defaultConfigurator) Watch(cb WatchCallbackFunc, names ...string) {
	w := &watcher{}
	w.names = make(map[string]struct{}, len(names))
	w.callback = cb

	for _, name := range names {
		w.names[name] = struct{}{}
	}

	c.rw.Lock()
	c.watchers = append(c.watchers, w)
	c.rw.Unlock()
}

// Load loads the configurations of the named source; empty file means every file of the source.
func (c *defaultConfigurator) Load(ctx context.Context, source string, file ...string) ([]*Configuration, error) {
	s, ok := c.sources[source]
	if !ok {
		return nil, errors.ErrNotFoundConfigSource
	}

	configs, err := s.Load(ctx, file...)
	if err != nil {
		return nil, err
	}

	for _, cc := range configs {
		cc.decoder, cc.scanner = c.opts.decoder, c.opts.scanner
	}

	return configs, nil
}

// Store saves content as file under the named source, merging it with the existing config unless
// override is true.
func (c *defaultConfigurator) Store(ctx context.Context, source string, file string, content any, override ...bool) error {
	if content == nil {
		return errors.ErrInvalidConfigContent
	}

	s, ok := c.sources[source]
	if !ok {
		return errors.ErrNotFoundConfigSource
	}

	var (
		err    error
		buf    []byte
		ext    = filepath.Ext(file)
		format = strings.TrimPrefix(ext, ".")
	)

	switch rk, _ := xreflect.Value(content); rk {
	case reflect.Map, reflect.Struct:
		if len(override) > 0 && override[0] {
			buf, err = c.opts.encoder(format, content)
		} else {
			dest, err := c.copy()
			if err != nil {
				return err
			}

			name := strings.TrimSuffix(filepath.Base(file), ext)

			val, ok := dest[name]
			if !ok {
				buf, err = c.opts.encoder(format, content)
			} else if v, ok := val.(map[string]any); ok {
				buf, err = c.opts.encoder(format, content)
				if err != nil {
					return err
				}

				maps, err := c.opts.decoder(format, buf)
				if err != nil {
					return err
				}

				err = mergo.Merge(&v, maps, mergo.WithOverride)
				if err != nil {
					return err
				}

				buf, err = c.opts.encoder(format, v)
			} else {
				buf, err = c.opts.encoder(format, content)
			}
		}
	case reflect.Array, reflect.Slice:
		buf, err = c.opts.encoder(format, content)
	default:
		buf = xconv.Bytes(xconv.String(content))
	}
	if err != nil {
		return err
	}

	return s.Store(ctx, file, buf)
}

func reviseKeys(keys []string, values map[string]any) []string {
	for i := 1; i < len(keys); i++ {
		key := strings.Join(keys[:i+1], ".")
		if _, ok := values[key]; ok {
			keys[0] = key
			temp := keys[i+1:]
			copy(keys[1:], temp)
			keys = keys[:len(temp)+1]
			break
		}
	}

	return keys
}
