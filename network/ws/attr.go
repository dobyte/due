package ws

import "sync"

type attr struct {
	values sync.Map
}

// Get returns the value of the attribute with the given key and whether it exists.
func (a *attr) Get(key any) (any, bool) {
	return a.values.Load(key)
}

// Set sets the value of the attribute with the given key.
func (a *attr) Set(key, value any) {
	a.values.Store(key, value)
}

// Del deletes the attribute with the given key and reports whether the deletion succeeded.
func (a *attr) Del(key any) (ok bool) {
	_, ok = a.values.LoadAndDelete(key)
	return
}

// Clear removes all attributes.
func (a *attr) Clear() {
	a.values.Clear()
}

// Visit calls fn for every attribute and stops when fn returns false.
func (a *attr) Visit(fn func(key, value any) bool) {
	a.values.Range(fn)
}
