package xreflect

import (
	"reflect"
)

// Value returns the reflect kind and value of val, dereferencing any pointer it holds.
func Value(val any) (reflect.Kind, reflect.Value) {
	var (
		rv = reflect.ValueOf(val)
		rk = rv.Kind()
	)

	for rk == reflect.Ptr {
		rv = rv.Elem()
		rk = rv.Kind()
	}

	return rk, rv
}

// IsNil reports whether val is nil, including a nil chan, func, map, pointer, unsafe pointer,
// interface or slice.
func IsNil(val any) bool {
	if val == nil {
		return true
	}

	rv := reflect.ValueOf(val)
	rk := rv.Kind()

	switch rk {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
