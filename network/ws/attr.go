package ws

import "sync"

type attr struct {
	values sync.Map
}

// Get 获取属性值
// @param key any 属性键
// @return @1 any 属性值
// @return @2 bool 是否存在
func (a *attr) Get(key any) (any, bool) {
	return a.values.Load(key)
}

// Set 设置属性值
// @param key any 属性键
// @param value any 属性值
func (a *attr) Set(key, value any) {
	a.values.Store(key, value)
}

// Del 删除属性值
// @param key any 属性键
// @return @1 bool 是否删除成功
func (a *attr) Del(key any) (ok bool) {
	_, ok = a.values.LoadAndDelete(key)
	return
}

// Clear 清空所有属性值
func (a *attr) Clear() {
	a.values.Clear()
}

// Visit 访问所有的属性值
// @param fn func(key, value any) bool 遍历函数，返回 false 时停止遍历
func (a *attr) Visit(fn func(key, value any) bool) {
	a.values.Range(fn)
}
