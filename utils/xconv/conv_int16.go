package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Int16 将任意值转换为 int16
// @param val any 待转换的值
// @return @1 int16 转换后的 int16
func Int16(val any) int16 {
	return int16(Int64(val))
}

// Int16s 将任意值转换为 int16 切片
// @param val any 待转换的值
// @return @1 []int16 转换后的 int16 切片
func Int16s(val any) (slice []int16) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]int:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []int8:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]int8:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []int16:
		return v
	case *[]int16:
		if v == nil {
			return
		}
		return *v
	case []int32:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]int32:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []int64:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]int64:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []uint:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]uint:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []uint8:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]uint8:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []uint16:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]uint16:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []uint32:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]uint32:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []uint64:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]uint64:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []float32:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]float32:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []float64:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]float64:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []complex64:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]complex64:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []complex128:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]complex128:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []string:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]string:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []bool:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]bool:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []any:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]any:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case [][]byte:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[][]byte:
		if v == nil {
			return
		}
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]int16, count)
			for i := range count {
				slice[i] = Int16(rv.Index(i).Interface())
			}
		}
	}

	return
}

// Int16Pointer 将任意值转换为 int16 指针
// @param val any 待转换的值
// @return @1 *int16 转换后的 int16 指针
func Int16Pointer(any any) *int16 {
	v := Int16(any)
	return &v
}

// Int16sPointer 将任意值转换为 int16 切片指针
// @param val any 待转换的值
// @return @1 *[]int16 转换后的 int16 切片指针
func Int16sPointer(any any) *[]int16 {
	v := Int16s(any)
	return &v
}
