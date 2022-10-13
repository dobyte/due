package config

import (
	"bytes"
	"encoding/gob"
	"github.com/dobyte/due/log"
	"strconv"
	"strings"
	"sync/atomic"
)

type Reader interface {
	// Get 获取配置值
	Get(pattern string, def ...interface{}) *Value
	// Set 设置配置值
	Set(pattern string, value interface{})
}

func init() {
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}

type defaultReader struct {
	opts   *options
	values atomic.Value
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

	r := &defaultReader{opts: o}
	r.init()

	return r
}

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

// Get 获取配置值
func (r *defaultReader) Get(pattern string, def ...interface{}) *Value {
	var (
		keys   = strings.Split(pattern, ".")
		values interface{}
		found  = true
	)

	values, err := r.copyValues()
	if err != nil {
		log.Errorf("copy configurations failed: %v", err)
		goto NOTFOUND
	}

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

NOTFOUND:
	if len(def) > 0 {
		return &Value{val: def[0]}
	}

	return &Value{val: nil}
}

// Set 设置配置值
func (r *defaultReader) Set(pattern string, value interface{}) {
	var (
		keys   = strings.Split(pattern, ".")
		values interface{}
	)

	src, err := r.copyValues()
	if err != nil {
		log.Errorf("copy configurations failed: %v", err)
		return
	}

	values = src
	for i, key := range keys {
		switch vs := values.(type) {
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

				values = vs[key]
			}
		case []interface{}:
			ii, err := strconv.Atoi(key)
			if err != nil {
				return
			}

			if ii >= len(vs) {
				return
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

				values = vs[ii]
			}
		}
	}

	r.values.Store(src)
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
