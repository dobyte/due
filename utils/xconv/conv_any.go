package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

func Anys(val any) []any {
	if val == nil {
		return nil
	}

	switch rk, rv := xreflect.Value(val); rk {
	case reflect.Slice, reflect.Array:
		count := rv.Len()
		slice := make([]any, count)
		for i := range count {
			slice[i] = rv.Index(i).Interface()
		}
		return slice
	default:
		return nil
	}
}

func AnysPointer(val any) *[]any {
	v := Anys(val)
	return &v
}
