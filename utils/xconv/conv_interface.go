package xconv

import "reflect"

func Interfaces(any interface{}) (slice []interface{}) {
	if any == nil {
		return
	}

	var (
		rv   = reflect.ValueOf(any)
		kind = rv.Kind()
	)

	for kind == reflect.Ptr {
		rv = rv.Elem()
		kind = rv.Kind()
	}

	switch kind {
	case reflect.Slice, reflect.Array:
		count := rv.Len()
		slice = make([]interface{}, count)
		for i := 0; i < count; i++ {
			slice[i] = Int(rv.Index(i).Interface())
		}
	}

	return
}

func InterfacesPointer(any interface{}) *[]interface{} {
	v := Interfaces(any)
	return &v
}
