package xreflect

import (
	"reflect"
)

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

// IsNil 检测值是否为nil
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
