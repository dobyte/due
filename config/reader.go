package config

import (
	"bytes"
	"context"
	"encoding/gob"
	"github.com/dobyte/due/env"
	"github.com/dobyte/due/errors"
	"github.com/dobyte/due/internal/value"
	"github.com/imdario/mergo"
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
	// Close 关闭读取器
	Close()
}

func init() {
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}

type defaultReader struct {
	opts   *options
	ctx    context.Context
	cancel context.CancelFunc
	values atomic.Value
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

	if len(o.sources) == 0 {
		path := env.Get(dueConfigEnvName, defaultConfigPath).String()
		o.sources = append(o.sources, NewSource(path))
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
		cs, err := s.Load()
		if err != nil {
			log.Fatalf("load configure failed: %v", err)
		}

		for _, c := range cs {
			v, err := r.opts.decoder(c)
			if err != nil {
				log.Fatalf("decode configure failed: %v", err)
			}

			values[c.Name] = v
		}
	}

	r.values.Store(values)
}

// 监听配置源变化
func (r *defaultReader) watch() {
	for _, s := range r.opts.sources {
		watcher, err := s.Watch(r.ctx)
		if err != nil {
			log.Fatalf("watching configure change failed: %v", err)
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
					v, err := r.opts.decoder(c)
					if err != nil {
						continue
					}
					values[c.Name] = v
				}

				dst, err := r.copyValues()
				if err != nil {
					continue
				}

				err = mergo.Merge(&dst, values, mergo.WithOverride)
				if err != nil {
					continue
				}

				r.values.Store(dst)
			}
		}()
	}
}

// Close 关闭读取器
func (r *defaultReader) Close() {
	r.cancel()
}

// Has 是否存在配置
func (r *defaultReader) Has(pattern string) bool {
	var (
		keys  = strings.Split(pattern, ".")
		node  interface{}
		found = true
	)

	values, err := r.copyValues()
	if err != nil {
		return false
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

	return found
}

// Get 获取配置值
func (r *defaultReader) Get(pattern string, def ...interface{}) value.Value {
	var (
		keys  = strings.Split(pattern, ".")
		node  interface{}
		found = true
	)

	values, err := r.copyValues()
	if err != nil {
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

	values, err := r.copyValues()
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

	r.values.Store(values)

	return nil
}

func (r *defaultReader) copyValues() (map[string]interface{}, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	dec := gob.NewDecoder(&buf)
	err := enc.Encode(r.values.Load())
	if err != nil {
		return nil, err
	}
	var dest map[string]interface{}
	err = dec.Decode(&dest)
	if err != nil {
		return nil, err
	}
	return dest, nil
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
