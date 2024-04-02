package config

import (
	"context"
	"github.com/apolloconfig/agollo/v4/storage"
	"github.com/imdario/mergo"
	"github.com/jinzhu/copier"
	"github.com/sasha-s/go-deadlock"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/value"
	"log"
	"strconv"
	"strings"
	"sync/atomic"
)

type Reader interface {
	// Has 是否存在配置
	Has(pattern string) bool
	// Get 获取配置值
	Get(pattern string, def ...interface{}) value.Value
	// Set 设置配置值
	Set(pattern string, value interface{}) error
	// AddChangeListener 设置配置变更监听
	AddChangeListener(listener interface{})
	// RemoveChangeListener 设置配置变更监听
	RemoveChangeListener(listener interface{})
	// Close 关闭配置监听
	Close()
}

const (
	Apollo = "apollo"

	defaultApolloEnable = "true"

	defaultApolloAppIdKey  = "config.apollo.appId"
	defaultApolloHostKey   = "config.apollo.host"
	defaultApolloPortKey   = "config.apollo.port"
	defaultApolloEnableKey = "config.apollo.enable"
)

type defaultReader struct {
	opts   *options
	ctx    context.Context
	cancel context.CancelFunc
	mu     deadlock.Mutex
	idx    int64
	values [2]map[string]interface{}
}

var _ Reader = &defaultReader{}

func NewReader(opts ...Option) Reader {
	o := &options{
		ctx:     context.Background(),
		decoder: defaultDecoder,
	}
	for _, opt := range opts {
		opt(o)
	}

	r := &defaultReader{}
	r.opts = o
	r.ctx, r.cancel = context.WithCancel(o.ctx)
	r.init()
	r.watch()

	return r
}

// 初始化配置源
func (r *defaultReader) init() {
	values := make(map[string]interface{})
	for _, s := range r.opts.sources {
		if s == nil {
			continue
		}
		cs, err := s.Load()
		if err != nil {
			log.Printf("load configure failed: %v", err)
			continue
		}
		log.Printf("load configure success: %v", s.Path())

		for _, c := range cs {
			value := make(map[string]interface{})
			err := r.opts.decoder(c, &value)
			if err != nil {
				log.Printf("decode configure failed: %v", err)
				continue
			}
			values = mergeMaps(values, value)
		}
	}

	values = map[string]interface{}{"config": values}
	r.store(values)

	// set remote reader
	for _, rs := range r.opts.remoteSources {
		switch rs {
		case Apollo:
			apolloEnable := r.Get(defaultApolloEnableKey, defaultApolloEnable).Bool()
			if !apolloEnable {
				continue
			}
			apolloAppId := r.Get(defaultApolloAppIdKey).String()
			apolloHost := r.Get(defaultApolloHostKey).String()
			apolloPort := r.Get(defaultApolloPortKey).Int()
			apollo := NewApolloReader(WithAppId(apolloAppId), WithHost(apolloHost), WithPort(apolloPort))
			apollo.Range(func(key, value interface{}) bool {
				if stringKey, ok := key.(string); ok {
					r.Set(stringKey, value)
				}
				return true
			})
			apollo.client.AddChangeListener(&CustomChangeListener{defaultReader: r})
			r.opts.remoteReaders = append(r.opts.remoteReaders, apollo)
		default:
		}
	}
}

// 保存配置
func (r *defaultReader) store(values map[string]interface{}) {
	idx := atomic.AddInt64(&r.idx, 1) % int64(len(r.values))
	r.values[idx] = values
}

// 加载配置
func (r *defaultReader) load() map[string]interface{} {
	idx := atomic.LoadInt64(&r.idx) % int64(len(r.values))
	return r.values[idx]
}

// 拷贝配置
func (r *defaultReader) copy() (map[string]interface{}, error) {
	values := r.load()

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
func (r *defaultReader) watch() {
	for _, s := range r.opts.sources {
		if s == nil {
			continue
		}
		watcher, err := s.Watch(r.ctx)
		if err != nil {
			log.Printf("watching configure change failed: %v", err)
			continue
		}

		go func() {
			defer watcher.Stop()

			for {
				select {
				case <-r.ctx.Done():
					return
				default:
					// exec watch
				}
				cs, err := watcher.Next()
				if err != nil {
					continue
				}

				values := make(map[string]interface{})
				for _, c := range cs {
					err := r.opts.decoder(c, values)
					if err != nil {
						continue
					}
				}
				values = map[string]interface{}{"config": values}

				func() {
					r.mu.Lock()
					defer r.mu.Unlock()

					dst, err := r.copy()
					if err != nil {
						return
					}

					err = mergo.Merge(&dst, values, mergo.WithOverride)
					if err != nil {
						return
					}

					r.store(dst)
				}()
			}
		}()
	}
}

// AddChangeListener 设置配置变更监听
func (r *defaultReader) AddChangeListener(listener interface{}) {
	for _, r := range r.opts.remoteReaders {
		switch v := r.(type) {
		case *ApolloReader:
			if l, ok := listener.(storage.ChangeListener); ok {
				v.AddChangeListener(l)
			}
		default:
		}
	}
}

// RemoveChangeListener 取消配置变更监听
func (r *defaultReader) RemoveChangeListener(listener interface{}) {
	for _, r := range r.opts.remoteReaders {
		switch v := r.(type) {
		case *ApolloReader:
			if l, ok := listener.(storage.ChangeListener); ok {
				v.RemoveChangeListener(l)
			}
		default:
		}
	}
}

// Close 关闭配置监听
func (r *defaultReader) Close() {
	r.cancel()
}

// Has 是否存在配置
func (r *defaultReader) Has(pattern string) bool {
	var (
		keys   = strings.Split(pattern, ".")
		node   interface{}
		found  = true
		values = r.load()
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
func (r *defaultReader) Get(pattern string, def ...interface{}) value.Value {
	var (
		keys   = strings.Split(pattern, ".")
		node   interface{}
		found  = true
		values = r.load()
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
		return value.NewValue(node)
	}

NOTFOUND:
	return value.NewValue(def...)
}

// Set 设置配置值
func (r *defaultReader) Set(pattern string, value interface{}) error {
	var (
		keys = strings.Split(pattern, ".")
		node interface{}
	)

	r.mu.Lock()
	defer r.mu.Unlock()

	values, err := r.copy()
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

	r.store(values)

	return nil
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

func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	for k, v := range b {
		// If you use map[string]interface{}, ok is always false here.
		// Because yaml.Unmarshal will give you map[string]interface{}.
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := a[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					a[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		a[k] = v
	}
	return a
}

type CustomChangeListener struct {
	defaultReader *defaultReader
}

func (c *CustomChangeListener) OnChange(changeEvent *storage.ChangeEvent) {
	for key, value := range changeEvent.Changes {
		c.defaultReader.Set(key, value.NewValue)
	}
}

func (c *CustomChangeListener) OnNewestChange(event *storage.FullChangeEvent) {
	//write your code here
}
