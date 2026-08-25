package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Uint16 将任意值转换为 uint16
// @param val any 待转换的值
// @return @1 uint16 转换后的 uint16
func Uint16(val any) uint16 {
	return uint16(Uint64(val))
}

// Uint16s 将任意值转换为 uint16 切片
// @param val any 待转换的值
// @return @1 []uint16 转换后的 uint16 切片
func Uint16s(val any) (slice []uint16) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]int:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []int8:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]int8:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []int16:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]int16:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []int32:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]int32:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []int64:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]int64:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []uint:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]uint:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []uint8:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]uint8:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []uint16:
		return v
	case *[]uint16:
		if v == nil {
			return
		}
		return *v
	case []uint32:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]uint32:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []uint64:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]uint64:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []float32:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]float32:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []float64:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]float64:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []complex64:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]complex64:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []complex128:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]complex128:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []string:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]string:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []bool:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]bool:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []any:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]any:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case [][]byte:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[][]byte:
		if v == nil {
			return
		}
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]uint16, count)
			for i := range count {
				slice[i] = Uint16(rv.Index(i).Interface())
			}
		}
	}

	return
}

// Uint16Pointer 将任意值转换为 uint16 指针
// @param val any 待转换的值
// @return @1 *uint16 转换后的 uint16 指针
func Uint16Pointer(val any) *uint16 {
	v := Uint16(val)
	return &v
}

// Uint16sPointer 将任意值转换为 uint16 切片指针
// @param val any 待转换的值
// @return @1 *[]uint16 转换后的 uint16 切片指针
func Uint16sPointer(val any) *[]uint16 {
	v := Uint16s(val)
	return &v
}
