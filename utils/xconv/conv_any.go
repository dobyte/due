// Package xconv provides conversion utilities between arbitrary types, covering conversions among
// numeric, string, boolean, byte, rune, duration and storage-capacity types as well as slice and
// pointer conversions.
package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Anys converts val to a slice of any.
//
// Only slice and array types are supported; other types yield nil.
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

// AnysPointer converts val to a pointer to a slice of any.
func AnysPointer(val any) *[]any {
	v := Anys(val)
	return &v
}
