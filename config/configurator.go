package config

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dario.cat/mergo"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xreflect"
	"github.com/jinzhu/copier"
)

const (
	// watchMinBackoff is the initial delay before retrying a failed config watch.
	watchMinBackoff = time.Second
	// watchMaxBackoff is the maximum delay before retrying a failed config watch.
	watchMaxBackoff = 30 * time.Second
	// maxSliceIndex is the largest index that Set expands a config slice to, so that a mistyped
	// index cannot allocate an unbounded amount of memory.
	maxSliceIndex = 10000
)

// Configurator manages a set of config sources and exposes the config loaded from them.
//
// The value store is merge-only: a config that a source no longer provides stays available until the
// configurator is recreated. A changed config replaces its previous value as a whole, so a key
// removed from a config file is removed from the store as well.
type Configurator interface {
	// Has reports whether a config matching pattern exists.
	Has(pattern string) bool
	// Get returns the config value under pattern, falling back to def when the config is missing.
	// The returned value may reference the internal config maps and slices and must be treated as
	// read-only; modifying it corrupts the snapshot shared by every reader.
	Get(pattern string, def ...any) value.Value
	// Set sets the config value under pattern; a non-container value on the path is replaced when a
	// nested pattern is given.
	Set(pattern string, value any) error
	// Match returns a [Matcher] over the given patterns.
	Match(patterns ...string) Matcher
	// Watch registers cb to be called when one of the named configs changes; empty names means every
	// config. Callbacks run on a single goroutine and are never invoked concurrently; changes that
	// arrive while a callback runs are merged into the next call.
	Watch(cb WatchCallbackFunc, names ...string)
	// Load loads the configurations of the named source; empty file means every file of the source.
	Load(ctx context.Context, source string, file ...string) ([]*Configuration, error)
	// Store saves content as file under the named source, merging it with the existing config unless
	// override is true.
	Store(ctx context.Context, source string, file string, content any, override ...bool) error
	// Close closes the configurator and its config sources. It waits for a running callback to
	// finish and discards the notifications that are still pending; a callback must not call Close.
	Close()
}

// WatchCallbackFunc is called with the names of the configs that changed. The names are sorted and
// may combine several changes.
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
	values   atomic.Pointer[map[string]any]
	rw       sync.RWMutex
	watchers []*watcher
	notifyMu sync.Mutex
	pending  map[string]struct{}
	chNotify chan struct{}
	wg       sync.WaitGroup
}

var _ Configurator = &defaultConfigurator{}

// NewConfigurator returns a new [Configurator] configured with opts and starts watching the config
// sources for changes.
func NewConfigurator(opts ...Option) Configurator {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	o.sources = normalizeSources(o.sources)

	r := &defaultConfigurator{}
	r.opts = o
	r.ctx, r.cancel = context.WithCancel(o.ctx)
	r.watchers = make([]*watcher, 0)
	r.pending = make(map[string]struct{})
	r.chNotify = make(chan struct{}, 1)
	r.wg.Go(r.notifyLoop)
	r.init()
	r.watch()

	return r
}

// normalizeSources keeps the first source of every name and closes the duplicates, so that a
// duplicate source cannot silently shadow another one or leak its connection.
func normalizeSources(sources []Source) []Source {
	if len(sources) < 2 {
		return sources
	}

	var (
		seen   = make(map[string]struct{}, len(sources))
		unique = make([]Source, 0, len(sources))
	)

	for _, s := range sources {
		if s == nil {
			log.Printf("nil config source is ignored")
			continue
		}

		name := s.Name()
		if _, ok := seen[name]; ok {
			log.Printf("duplicate config source %q is ignored", name)

			if err := s.Close(); err != nil {
				log.Printf("close duplicate config source %q failed: %v", name, err)
			}

			continue
		}

		seen[name] = struct{}{}
		unique = append(unique, s)
	}

	return unique
}

// init initializes the config sources and loads their config into the value store. A source that
// fails to load is retried in the background, so that a temporary failure does not leave the store
// permanently incomplete.
func (c *defaultConfigurator) init() {
	c.sources = make(map[string]Source, len(c.opts.sources))
	for _, s := range c.opts.sources {
		c.sources[s.Name()] = s
	}

	for _, s := range c.opts.sources {
		cs, err := s.Load(c.ctx)
		if err != nil {
			if errors.Is(err, errors.ErrClosedConfigSource) {
				log.Printf("config source %q is closed, skip loading", s.Name())

				continue
			}

			log.Printf("load configure failed: %v", err)

			c.wg.Go(func() {
				c.reload(s)
			})

			continue
		}

		c.apply(cs)
	}
}

// reload retries loading the configurations of a source that failed at startup and merges them into
// the store once the source becomes available.
func (c *defaultConfigurator) reload(s Source) {
	delay := watchMinBackoff

	for {
		if !c.sleep(delay) {
			return
		}

		cs, err := s.Load(c.ctx)
		if err != nil {
			if c.ctx.Err() != nil {
				return
			}

			if errors.Is(err, errors.ErrClosedConfigSource) {
				log.Printf("config source %q is closed, stop reloading", s.Name())

				return
			}

			log.Printf("load configure failed: %v", err)

			delay = nextWatchDelay(delay)

			continue
		}

		if names := c.apply(cs); len(names) > 0 {
			c.scheduleNotify(names)
		}

		return
	}
}

// apply decodes the given configurations and stores them in the value store. The value of every
// applied config replaces its previous value as a whole, so a key removed from a changed config is
// removed from the store. It returns the names of the configurations that were applied.
func (c *defaultConfigurator) apply(cs []*Configuration) []string {
	names := make([]string, 0, len(cs))
	values := make(map[string]any, len(cs))

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

		names = append(names, cc.Name)
		values[cc.Name] = v
	}

	if len(values) == 0 {
		return names
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	dst := c.shallowCopy()

	for name, v := range values {
		dst[name] = v
	}

	c.store(dst)

	return names
}

// store publishes values as the current snapshot. The map must not be modified afterwards, so that
// readers loading the snapshot never observe a concurrent write.
func (c *defaultConfigurator) store(values map[string]any) {
	c.values.Store(&values)
}

// load returns the current snapshot.
func (c *defaultConfigurator) load() map[string]any {
	if values := c.values.Load(); values != nil {
		return *values
	}

	return nil
}

// copy returns a deep copy of the currently loaded values.
func (c *defaultConfigurator) copy() (map[string]any, error) {
	values := c.load()

	dst := make(map[string]any)

	if len(values) == 0 {
		return dst, nil
	}

	err := copier.CopyWithOption(&dst, values, copier.Option{
		DeepCopy: true,
	})
	if err != nil {
		return nil, err
	}

	return dst, nil
}

// shallowCopy returns a copy of the top-level value map. The nested values are shared with the
// current snapshot: every writer replaces a value as a whole instead of modifying it in place, so
// the shared subtrees stay immutable.
func (c *defaultConfigurator) shallowCopy() map[string]any {
	values := c.load()

	dst := make(map[string]any, len(values))

	for name, v := range values {
		dst[name] = v
	}

	return dst
}

// watch starts a goroutine per source that opens a watcher for it and merges its changes into the
// store.
func (c *defaultConfigurator) watch() {
	for _, s := range c.opts.sources {
		c.wg.Go(func() {
			c.watchSource(s)
		})
	}
}

// watchSource opens a watcher for the source and consumes its changes until the configurator is
// closed. Both a watcher that cannot be opened and one that stops are retried with a backoff, so
// that a temporary failure does not disable hot reload permanently.
func (c *defaultConfigurator) watchSource(s Source) {
	var (
		delay  = watchMinBackoff
		warned bool
	)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			// exec watch
		}

		w, err := s.Watch(c.ctx)
		if err == nil && w == nil {
			err = errors.New("nil config watcher")
		}
		if err != nil {
			if c.ctx.Err() != nil {
				return
			}

			// A closed source cannot be watched again; retrying would only log the same error on
			// every backoff.
			if errors.Is(err, errors.ErrClosedConfigSource) {
				log.Printf("config source %q is closed, stop watching", s.Name())

				return
			}

			if !warned {
				log.Printf("watching configure change failed: %v", err)
				warned = true
			}

			if !c.sleep(delay) {
				return
			}

			delay = nextWatchDelay(delay)

			continue
		}

		warned = false
		delay = watchMinBackoff

		warned, delay = c.consume(w, delay)
		if c.ctx.Err() != nil {
			return
		}

		// The watcher has stopped; reopen it after a backoff so that a watcher that fails
		// immediately is not reopened in a tight loop.
		if !c.sleep(delay) {
			return
		}

		delay = nextWatchDelay(delay)
	}
}

// consume reads the changes of a watcher until it stops or the configurator is closed. It returns
// the current warning state and the retry delay for the next watcher.
func (c *defaultConfigurator) consume(w Watcher, delay time.Duration) (bool, time.Duration) {
	defer w.Stop()

	var warned bool

	for {
		select {
		case <-c.ctx.Done():
			return warned, delay
		default:
			// exec watch
		}

		cs, err := w.Next()
		if err != nil {
			// A watcher reports context.Canceled once it has been stopped, so the configurator has
			// to open a new one instead of retrying the stopped watcher forever.
			if c.ctx.Err() != nil ||
				errors.Is(err, errors.ErrWatcherStopped) ||
				errors.Is(err, errors.ErrClosedConfigSource) ||
				errors.Is(err, context.Canceled) ||
				errors.Is(err, io.EOF) {
				return warned, delay
			}

			if !warned {
				log.Printf("watching configure change failed: %v", err)
				warned = true
			}

			if !c.sleep(delay) {
				return warned, delay
			}

			delay = nextWatchDelay(delay)

			continue
		}

		warned = false
		delay = watchMinBackoff

		if names := c.apply(cs); len(names) > 0 {
			c.scheduleNotify(names)
		}
	}
}

// sleep waits for delay or until the configurator is closed. It reports false when the configurator
// is closed.
func (c *defaultConfigurator) sleep(delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-c.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// nextWatchDelay doubles delay and caps it at watchMaxBackoff.
func nextWatchDelay(delay time.Duration) time.Duration {
	if delay >= watchMaxBackoff {
		return watchMaxBackoff
	}

	if delay *= 2; delay > watchMaxBackoff {
		delay = watchMaxBackoff
	}

	return delay
}

// scheduleNotify records the given names and wakes the notifier without blocking, so that a slow
// callback cannot stall the config sources.
func (c *defaultConfigurator) scheduleNotify(names []string) {
	c.notifyMu.Lock()
	for _, name := range names {
		c.pending[name] = struct{}{}
	}
	c.notifyMu.Unlock()

	select {
	case c.chNotify <- struct{}{}:
	default:
		// A pending notification signal is already queued, and the names are merged into the next
		// callback by notifyLoop.
	}
}

// notifyLoop delivers pending config names on a single goroutine until the configurator is closed.
// Callbacks are therefore never invoked concurrently, and changes that arrive while a callback runs
// are merged into the next call.
func (c *defaultConfigurator) notifyLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.chNotify:
		}

		if names := c.takePending(); len(names) > 0 {
			c.notify(names...)
		}
	}
}

// takePending drains the pending names in a stable order, so that a callback always observes the
// same argument order for the same set of changes.
func (c *defaultConfigurator) takePending() []string {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()

	if len(c.pending) == 0 {
		return nil
	}

	names := make([]string, 0, len(c.pending))
	for name := range c.pending {
		names = append(names, name)
	}

	clear(c.pending)

	slices.Sort(names)

	return names
}

// notify calls every watcher that matches one of the given names. The watcher list is snapshotted
// before the callbacks run, so a callback can call [Configurator.Watch] without deadlocking. A
// panicking callback is recovered so that it cannot terminate the process.
func (c *defaultConfigurator) notify(names ...string) {
	c.rw.RLock()
	watchers := make([]*watcher, len(c.watchers))
	copy(watchers, c.watchers)
	c.rw.RUnlock()

	for _, w := range watchers {
		if len(w.names) == 0 {
			callWatchCallback(w.callback, names...)
		} else {
			validNames := make([]string, 0, min(len(w.names), len(names)))
			for _, name := range names {
				if _, ok := w.names[name]; ok {
					validNames = append(validNames, name)
				}
			}

			if len(validNames) > 0 {
				callWatchCallback(w.callback, validNames...)
			}
		}
	}
}

// callWatchCallback invokes cb and recovers from its panic, so that a faulty watcher callback cannot
// terminate the process.
func callWatchCallback(cb WatchCallbackFunc, names ...string) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("watch callback panic: %v", err)
		}
	}()

	cb(names...)
}

// Close closes the configurator and its config sources. It waits for a running callback to finish
// and discards the notifications that are still pending.
func (c *defaultConfigurator) Close() {
	c.cancel()

	for _, source := range c.sources {
		_ = source.Close()
	}

	c.wg.Wait()
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
			if err != nil || i < 0 || i >= len(vs) {
				found = false
			} else {
				node = vs[i]
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

// Get returns the config value under pattern, falling back to def when the config is missing. The
// returned value must be treated as read-only, since it may reference the internal config maps and
// slices.
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
			if err != nil || i < 0 || i >= len(vs) {
				found = false
			} else {
				node = vs[i]
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
							if ii > maxSliceIndex {
								return errors.New("index too large")
							}

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

			if ii < 0 {
				return errors.New("invalid index")
			}

			if ii >= len(vs) {
				return errors.New("index overflow")
			}

			if i == len(keys)-1 {
				vs[ii] = value
			} else {
				rebuild := false
				next, parseErr := strconv.Atoi(keys[i+1])
				switch nv := vs[ii].(type) {
				case map[string]any:
					rebuild = parseErr == nil
				case []any:
					rebuild = parseErr != nil
					// the next node capacity is not enough
					// expand capacity
					if parseErr == nil && next >= len(nv) {
						if next > maxSliceIndex {
							return errors.New("index too large")
						}

						dst := make([]any, next+1)
						copy(dst, nv)
						vs[ii] = dst
					}
				default:
					rebuild = true
				}

				if rebuild {
					if parseErr != nil {
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
			dest, copyErr := c.copy()
			if copyErr != nil {
				return copyErr
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

				maps, decodeErr := c.opts.decoder(format, buf)
				if decodeErr != nil {
					return decodeErr
				}

				if err = mergo.Merge(&v, maps, mergo.WithOverride); err != nil {
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
