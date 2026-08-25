package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// GenericNumbers 将任意数值类型的切片/数组转换为指定泛型类型的数值切片
// 逐元素直接转换，目标类型支持整数、无符号整数、浮点数及基于它们的自定义类型，
// 不会经过 JSON 序列化，可避免大整数精度丢失；元素无法转换时转为零值
// @param val any 待转换的切片或数组
// @return @1 []T 转换后的数值切片
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

// toNumber 将任意值转换为泛型数值类型 T
// T 必须是整数、无符号整数或浮点类型（含基于它们的自定义类型），否则返回零值
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
