package config

import (
	"fmt"
	"github.com/dobyte/due/log"
	"strconv"
	"strings"
	"sync"
)

type Reader interface {
	// Get 获取配置值
	Get(pattern string, def ...interface{}) *Value
	// Set 设置配置值
	Set(pattern string, value interface{})
}

type defaultReader struct {
	err    error
	opts   *options
	rw     sync.RWMutex
	values map[string]interface{}
}

var _ Reader = &defaultReader{}

func NewReader(opts ...Option) Reader {
	o := &options{
		sources: []Source{NewSource("./config")},
		decoder: defaultDecoder,
	}
	for _, opt := range opts {
		opt(o)
	}

	r := &defaultReader{opts: o, values: make(map[string]interface{})}

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

			r.rw.Lock()
			r.values[c.Name] = v
			r.rw.Unlock()
		}
	}

	return r
}

func (r *defaultReader) Load(name ...string) *Value {
	r.rw.RLock()
	defer r.rw.RUnlock()

	return nil
}

// Get 获取配置值
func (r *defaultReader) Get(pattern string, def ...interface{}) *Value {
	r.rw.RLock()
	defer r.rw.RUnlock()

	var (
		keys   = strings.Split(pattern, ".")
		values interface{}
		found  = true
	)

	values = r.values
	for _, key := range keys {
		switch vs := values.(type) {
		case map[string]interface{}:
			if v, ok := vs[key]; ok {
				values = v
			} else {
				found = false
			}
		case []interface{}:
			i, err := strconv.Atoi(key)
			if err != nil {
				found = false
			} else if len(vs) > i {
				values = vs[i]
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
		return &Value{val: values}
	}

	if len(def) > 0 {
		return &Value{val: def[0]}
	}

	return &Value{val: nil}
}

// Set 设置配置值
func (r *defaultReader) Set(pattern string, value interface{}) {
	r.rw.Lock()
	defer r.rw.Unlock()

	var (
		keys   = strings.Split(pattern, ".")
		values interface{}
	)

	values = r.values
	for i, key := range keys {
		switch vs := values.(type) {
		case map[string]interface{}:
			if i == len(keys)-1 {
				vs[key] = value
			} else {
				rebuild := false
				_, err := strconv.Atoi(keys[i+1])
				if next, ok := vs[key]; ok {
					switch next.(type) {
					case map[string]interface{}:
						rebuild = err == nil
					case []interface{}:
						rebuild = err != nil
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
						vs[key] = make([]interface{}, 0)
					}
				}

				values = vs[key]
			}
		case []interface{}:
			ii, err := strconv.Atoi(key)
			if err != nil {
				return
			}

			if ii >= len(vs) {
				vs = append(vs, struct{}{})
				ii = len(vs) - 1
				fmt.Println(vs[ii])
			}

			if i == len(keys)-1 {
				vs[ii] = value
			} else {
				rebuild := false
				_, err = strconv.Atoi(keys[i+1])
				switch vs[ii].(type) {
				case map[string]interface{}:
					rebuild = err == nil
				case []interface{}:
					rebuild = err != nil
				default:
					rebuild = true
				}

				if rebuild {
					if err != nil {
						vs[ii] = make(map[string]interface{})
					} else {
						vs[ii] = make([]interface{}, 0)
					}
				}

				values = vs[ii]
			}
		}
	}

	fmt.Println(r.values)
}
