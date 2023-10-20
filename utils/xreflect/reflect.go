package xreflect

import "reflect"

func Value(i any) (reflect.Kind, reflect.Value) {
	var (
		rv = reflect.ValueOf(i)
		rk = rv.Kind()
	)

	for rk == reflect.Ptr {
		rv = rv.Elem()
		rk = rv.Kind()
	}

	return rk, rv
}
