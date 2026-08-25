package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/utils/xreflect"
)

// Json 将任意值转换为 JSON 字符串
// 对已满足 JSON 格式（以 { } 或 [ ] 包裹）的字符串/字节切片直接返回；
// 对 map、数组、切片、结构体等类型进行 JSON 序列化；其他类型返回空字符串
// @param val any 待转换的值
// @return @1 string 转换后的 JSON 字符串
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
