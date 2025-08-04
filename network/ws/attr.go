package ws

import "sync"

type attr struct {
	values sync.Map
}

// Get 获取属性值
func (a *attr) Get(key any) (any, bool) {
	return a.values.Load(key)
}

// Set 设置属性值
func (a *attr) Set(key, value any) {
	a.values.Store(key, value)
}

// Del 删除属性值
func (a *attr) Del(key any) (ok bool) {
	_, ok = a.values.LoadAndDelete(key)
	return
}

// Visit 访问所有的属性值
func (a *attr) Visit(fn func(key, value any) bool) {
	a.values.Range(fn)
}
