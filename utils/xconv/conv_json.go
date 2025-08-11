package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/utils/xreflect"
)

func Json(val any) string {
	isJson := func(s string) bool {
		l := len(s)
		return l >= 2 && ((s[0] == '{' && s[l-1] == '}') || (s[0] == '[' && s[l-1] == ']'))
	}

	switch v := val.(type) {
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
		switch rk, rv := xreflect.Value(val); rk {
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
