package kcp

import "sync"

type attr struct {
	values sync.Map
}

// Get 获取属性值
// @param key any 属性键
// @return @1 any 属性值，未找到时为nil
// @return @2 bool 属性是否存在
func (a *attr) Get(key any) (any, bool) {
	return a.values.Load(key)
}

// Set 设置属性值
// 若键已存在则覆盖原值
// @param key any 属性键
// @param value any 属性值
func (a *attr) Set(key, value any) {
	a.values.Store(key, value)
}

// Del 删除属性值
// @param key any 属性键
// @return @1 bool 删除操作是否成功（键不存在时为false）
func (a *attr) Del(key any) (ok bool) {
	_, ok = a.values.LoadAndDelete(key)
	return
}

// Clear 清空所有属性值
func (a *attr) Clear() {
	a.values.Clear()
}

// Visit 访问所有的属性值
// 遍历过程中会对所有属性依次执行 fn，当 fn 返回 false 时提前终止遍历
// @param fn func(key, value any) bool 属性访问函数，返回 false 时终止遍历
func (a *attr) Visit(fn func(key, value any) bool) {
	a.values.Range(fn)
}
