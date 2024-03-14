package config

import (
	"context"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xreflect"
	"github.com/imdario/mergo"
	"github.com/jinzhu/copier"
	"log"
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type Configurator interface {
	// Has 检测多个匹配规则中是否存在配置
	Has(pattern string) bool
	// Get 获取配置值
	Get(pattern string, def ...interface{}) value.Value
	// Set 设置配置值
	Set(pattern string, value interface{}) error
	// Match 匹配多个规则
	Match(patterns ...string) Matcher
	// Watch 设置监听回调
	Watch(cb WatchCallbackFunc, names ...string)
	// Load 加载配置项
	Load(ctx context.Context, source string, file ...string) ([]*Configuration, error)
	// Store 保存配置项
	Store(ctx context.Context, source string, file string, content interface{}, override ...bool) error
	// Close 关闭配置监听
	Close()
}

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
	idx      int64
	values   [2]map[string]interface{}
	rw       sync.RWMutex
	watchers []*watcher
}

var _ Configurator = &defaultConfigurator{}

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

// 初始化配置源
func (c *defaultConfigurator) init() {
	c.sources = make(map[string]Source, len(c.opts.sources))
	for _, s := range c.opts.sources {
		c.sources[s.Name()] = s
	}

	values := make(map[string]interface{})
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
				if err != errors.ErrInvalidFormat {
					log.Printf("decode configure failed: %v", err)
				}
				continue
			}

			values[cc.Name] = v
		}
	}

	c.store(values)
}

// 保存配置
func (c *defaultConfigurator) store(values map[string]interface{}) {
	idx := atomic.AddInt64(&c.idx, 1) % int64(len(c.values))
	c.values[idx] = values
}

// 加载配置
func (c *defaultConfigurator) load() map[string]interface{} {
	idx := atomic.LoadInt64(&c.idx) % int64(len(c.values))
	return c.values[idx]
}

// 拷贝配置
func (c *defaultConfigurator) copy() (map[string]interface{}, error) {
	values := c.load()

	dst := make(map[string]interface{})

	err := copier.CopyWithOption(&dst, values, copier.Option{
		DeepCopy: true,
	})
	if err != nil {
		return nil, err
	}

	return dst, nil
}

// 监听配置源变化
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
				values := make(map[string]interface{})
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

// 通知给监听器
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

// Close 关闭配置监听
func (c *defaultConfigurator) Close() {
	c.cancel()

	for _, source := range c.sources {
		_ = source.Close()
	}
}

// Has 检测多个匹配规则中是否存在配置
func (c *defaultConfigurator) Has(pattern string) bool {
	return c.doHas(pattern)
}

// 执行检测配置是否存在操作
func (c *defaultConfigurator) doHas(pattern string) bool {
	var (
		keys   = strings.Split(pattern, ".")
		node   interface{}
		found  = true
		values = c.load()
	)

	keys = reviseKeys(keys, values)
	node = values
	for _, key := range keys {
		switch vs := node.(type) {
		case map[string]interface{}:
			if v, ok := vs[key]; ok {
				node = v
			} else {
				found = false
			}
		case []interface{}:
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

// Get 获取配置值
func (c *defaultConfigurator) Get(pattern string, def ...interface{}) value.Value {
	if val, ok := c.doGet(pattern); ok {
		return val
	}

	return value.NewValue(def...)
}

// Match 匹配多个规则
func (c *defaultConfigurator) Match(patterns ...string) Matcher {
	return &defaultMatcher{c: c, patterns: patterns}
}

// 执行获取配置操作
func (c *defaultConfigurator) doGet(pattern string) (value.Value, bool) {
	var (
		keys   = strings.Split(pattern, ".")
		node   interface{}
		found  = true
		values = c.load()
	)

	if values == nil || len(values) == 0 {
		goto NOTFOUND
	}

	keys = reviseKeys(keys, values)
	node = values
	for _, key := range keys {
		switch vs := node.(type) {
		case map[string]interface{}:
			if v, ok := vs[key]; ok {
				node = v
			} else {
				found = false
			}
		case []interface{}:
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

// Set 设置配置值
func (c *defaultConfigurator) Set(pattern string, value interface{}) error {
	var (
		keys = strings.Split(pattern, ".")
		node interface{}
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
		case map[string]interface{}:
			if i == len(keys)-1 {
				vs[key] = value
			} else {
				rebuild := false
				ii, err := strconv.Atoi(keys[i+1])
				if next, ok := vs[key]; ok {
					switch nv := next.(type) {
					case map[string]interface{}:
						rebuild = err == nil
					case []interface{}:
						rebuild = err != nil
						// the next node capacity is not enough
						// expand capacity
						if err == nil && ii >= len(nv) {
							dst := make([]interface{}, ii+1)
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
						vs[key] = make(map[string]interface{})
					} else {
						vs[key] = make([]interface{}, 1)
					}
				}

				node = vs[key]
			}
		case []interface{}:
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
				case map[string]interface{}:
					rebuild = err == nil
				case []interface{}:
					rebuild = err != nil
					// the next node capacity is not enough
					// expand capacity
					if err == nil && ii >= len(nv) {
						dst := make([]interface{}, ii+1)
						copy(dst, nv)
						vs[ii] = dst
					}
				default:
					rebuild = true
				}

				if rebuild {
					if err != nil {
						vs[ii] = make(map[string]interface{})
					} else {
						vs[ii] = make([]interface{}, 1)
					}
				}

				node = vs[ii]
			}
		}
	}

	c.store(values)

	return nil
}

// Watch 设置监听回调
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

// Load 加载配置项
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
		cc.decoder = c.opts.decoder
	}

	return configs, nil
}

// Store 保存配置项
func (c *defaultConfigurator) Store(ctx context.Context, source string, file string, content interface{}, override ...bool) error {
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
			} else if v, ok := val.(map[string]interface{}); ok {
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

func reviseKeys(keys []string, values map[string]interface{}) []string {
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
