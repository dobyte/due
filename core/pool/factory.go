package pool

import (
	"sync"

	"golang.org/x/sync/singleflight"
)

type Factory[T any] struct {
	ins sync.Map
	sfg singleflight.Group
	new func(name string) (T, error)
}

// NewFactory 创建单例工厂
func NewFactory[T any](new func(name string) (T, error)) *Factory[T] {
	return &Factory[T]{
		new: new,
	}
}

// Get 获取单例对象
func (f *Factory[T]) Get(name string) (T, error) {
	if val, ok := f.ins.Load(name); ok {
		return val.(T), nil
	}

	var zero T

	val, err, _ := f.sfg.Do(name, func() (any, error) {
		if val, ok := f.ins.Load(name); ok {
			return val, nil
		}

		val, err := f.new(name)
		if err != nil {
			return zero, err
		}

		f.ins.Store(name, val)

		return val, nil
	})
	if err != nil {
		return zero, err
	}

	return val.(T), nil
}
