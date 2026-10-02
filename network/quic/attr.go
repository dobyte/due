package quic

import "sync"

type attr struct {
	values sync.Map
}

// Get returns the attribute value stored under key. The value is nil when key does not exist.
//
// The second result reports whether key exists.
func (a *attr) Get(key any) (any, bool) {
	return a.values.Load(key)
}

// Set stores value under key.
func (a *attr) Set(key, value any) {
	a.values.Store(key, value)
}

// Del removes the attribute stored under key and reports whether it existed before deletion.
func (a *attr) Del(key any) (ok bool) {
	_, ok = a.values.LoadAndDelete(key)
	return
}

// Clear removes every attribute.
func (a *attr) Clear() {
	a.values.Clear()
}

// Visit calls fn for every attribute and stops when fn returns false.
func (a *attr) Visit(fn func(key, value any) bool) {
	a.values.Range(fn)
}
