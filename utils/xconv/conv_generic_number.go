package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// GenericNumbers converts a slice or array of arbitrary numeric values to a slice of the generic
// numeric type T.
//
// Elements are converted directly; T may be an integer, unsigned integer, floating-point or a
// named type based on them. The conversion does not go through JSON serialization and therefore
// avoids precision loss for large integers; an element that cannot be converted becomes the zero
// value.
func GenericNumbers[T any](val any) (slice []T) {
	if val == nil {
		return
	}

	switch rk, rv := xreflect.Value(val); rk {
	case reflect.Slice, reflect.Array:
		count := rv.Len()
		slice = make([]T, count)
		for i := range count {
			slice[i] = toNumber[T](rv.Index(i).Interface())
		}
	}

	return
}

// toNumber converts val to the generic numeric type T.
//
// T must be an integer, unsigned integer or floating-point type (including named types based on
// them), otherwise the zero value is returned.
func toNumber[T any](val any) T {
	rv := reflect.New(reflect.TypeOf((*T)(nil)).Elem()).Elem()

	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		rv.SetInt(Int64(val))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		rv.SetUint(Uint64(val))
	case reflect.Float32, reflect.Float64:
		rv.SetFloat(Float64(val))
	default:
		return reflect.Zero(reflect.TypeOf((*T)(nil)).Elem()).Interface().(T)
	}

	return rv.Interface().(T)
}
