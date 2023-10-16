package configurator

import (
	"context"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/imdario/mergo"
	"github.com/jinzhu/copier"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	ErrInvalidDecoder        = errors.New("invalid decoder")
	ErrNoOperationPermission = errors.New("no operation permission")
	ErrInvalidConfigContent  = errors.New("invalid config content")
	ErrNotFoundConfigSource  = errors.New("not found config source")
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
	Watch(cb WatchCallbackFunc)
	// Load 加载配置项
	Load(ctx context.Context, source string, file ...string) ([]*Configuration, error)
	// Store 保存配置项
	Store(ctx context.Context, source string, file string, content interface{}) error
	// Close 关闭配置监听
	Close()
}

type Source interface {
	// Name 配置源名称
	Name() string
	// Load 加载配置项
	Load(ctx context.Context, file ...string) ([]*Configuration, error)
	// Store 保存配置项
	Store(ctx context.Context, file string, content []byte) error
	// Watch 监听配置项
	Watch(ctx context.Context) (Watcher, error)
}

type Watcher interface {
	// Next 返回服务实例列表
	Next() ([]*Configuration, error)
	// Stop 停止监听
	Stop() error
}

type WatchCallbackFunc func(names ...string)

// Configuration 配置项
type Configuration struct {
	decoder  Decoder // 解码器
	scanner  Scanner // 扫描器
	Path     string  // 文件路径
	File     string  // 文件名称
	Name     string  // 文件名称
	Format   string  // 文件格式
	Content  []byte  // 文件内容
	FullPath string  // 文件全路径
}

// Decode 解码
func (c *Configuration) Decode() (interface{}, error) {
	if c.decoder == nil {
		return nil, ErrInvalidDecoder
	}

	return c.decoder(c.Format, c.Content)
}

// Scan 扫描
func (c *Configuration) Scan(dest interface{}) error {
	if c.scanner == nil {
		return ErrInvalidDecoder
	}

	return c.scanner(c.Format, c.Content, dest)
}

type defaultConfigurator struct {
	opts      *options
	ctx       context.Context
	cancel    context.CancelFunc
	sources   map[string]Source
	mu        sync.Mutex
	idx       int64
	values    [2]map[string]interface{}
	rw        sync.RWMutex
	callbacks []WatchCallbackFunc
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
	r.callbacks = make([]WatchCallbackFunc, 0)
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
				log.Printf("decode configure failed: %v", err)
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
		watcher, err := s.Watch(c.ctx)
		if err != nil {
			log.Printf("watching configure change failed: %v", err)
			continue
		}

		go func() {
			defer watcher.Stop()

			for {
				select {
				case <-c.ctx.Done():
					return
				default:
					// exec watch
				}
				cs, err := watcher.Next()
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
					c.rw.RLock()
					for _, cb := range c.callbacks {
						cb(names...)
					}
					c.rw.RUnlock()
				}
			}
		}()
	}
}

// Close 关闭配置监听
func (c *defaultConfigurator) Close() {
	c.cancel()
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

	if values == nil {
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
func (c *defaultConfigurator) Watch(cb WatchCallbackFunc) {
	c.rw.Lock()
	c.callbacks = append(c.callbacks, cb)
	c.rw.Unlock()
}

// Load 加载配置项
func (c *defaultConfigurator) Load(ctx context.Context, source string, file ...string) ([]*Configuration, error) {
	s, ok := c.sources[source]
	if !ok {
		return nil, ErrNotFoundConfigSource
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
func (c *defaultConfigurator) Store(ctx context.Context, source string, file string, content interface{}) error {
	if content == nil {
		return ErrInvalidConfigContent
	}

	s, ok := c.sources[source]
	if !ok {
		return ErrNotFoundConfigSource
	}

	var (
		val    []byte
		err    error
		format = strings.TrimPrefix(filepath.Ext(file), ".")
	)

	switch content.(type) {
	case map[string]interface{}:
		val, err = c.opts.encoder(format, content)
	case []interface{}:
		val, err = c.opts.encoder(format, content)
	default:
		val = xconv.Bytes(content)
	}
	if err != nil {
		return err
	}

	return s.Store(ctx, file, val)
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
