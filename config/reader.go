package config

import (
	"github.com/dobyte/due/log"
	"strconv"
	"strings"
	"sync"
)

type Reader interface {
	// Get 获取配置值
	Get(pattern string, def ...interface{}) *Value
	// Set 设置配置值
	Set(pattern string, value string)
}

type defaultReader struct {
	err    error
	opts   *options
	rw     sync.RWMutex
	values map[string]interface{}
}

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
func (r *defaultReader) Set(pattern string, value string) {

}
