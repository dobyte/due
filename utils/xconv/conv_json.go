package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/utils/xreflect"
)

// Json converts val to a JSON string.
//
// A string or byte slice that already looks like JSON (wrapped in { } or [ ]) is returned as is;
// maps, arrays, slices, structs and the like are JSON-marshalled; other types yield an empty
// string.
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
		if v == nil {
			return ""
		}
		if isJson(*v) {
			return *v
		}
	case []byte:
		if s := BytesToString(v); isJson(s) {
			return s
		}
	case *[]byte:
		if v == nil {
			return ""
		}
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
