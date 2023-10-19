package xconv

import (
	"github.com/dobyte/due/v2/encoding/json"
	"reflect"
)

func Json(any interface{}) string {
	isJson := func(s string) bool {
		l := len(s)
		return l >= 2 && ((s[0] == '{' && s[l-1] == '}') || (s[0] == '[' && s[l-1] == ']'))
	}

	switch v := any.(type) {
	case string:
		if isJson(v) {
			return v
		}
	case *string:
		if isJson(*v) {
			return *v
		}
	case []byte:
		if s := BytesToString(v); isJson(s) {
			return s
		}
	case *[]byte:
		if s := BytesToString(*v); isJson(s) {
			return s
		}
	default:
		var (
			rv   = reflect.ValueOf(any)
			kind = rv.Kind()
		)

		for kind == reflect.Ptr {
			rv = rv.Elem()
			kind = rv.Kind()
		}

		switch kind {
		case reflect.String:
			if s := rv.String(); isJson(s) {
				return s
			}
		case reflect.Map, reflect.Array, reflect.Slice, reflect.Struct:
			if b, err := json.Marshal(v); err == nil {
				return BytesToString(b)
			}
		}
	}

	return ""
}
