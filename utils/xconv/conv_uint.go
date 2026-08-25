package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Uint 将任意值转换为 uint
// @param val any 待转换的值
// @return @1 uint 转换后的 uint
func Uint(val any) uint {
	return uint(Uint64(val))
}

// Uints 将任意值转换为 uint 切片
// @param val any 待转换的值
// @return @1 []uint 转换后的 uint 切片
func Uints(val any) (slice []uint) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]int:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []int8:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]int8:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []int16:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]int16:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []int32:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]int32:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []int64:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]int64:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []uint:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]uint:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []uint8:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]uint8:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []uint16:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]uint16:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []uint32:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]uint32:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []uint64:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]uint64:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []float32:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]float32:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []float64:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]float64:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []complex64:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]complex64:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []complex128:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]complex128:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []string:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]string:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []bool:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]bool:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []any:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]any:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case [][]byte:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[][]byte:
		if v == nil {
			return
		}
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]uint, count)
			for i := range count {
				slice[i] = Uint(rv.Index(i).Interface())
			}
		}
	}

	return
}

// UintPointer 将任意值转换为 uint 指针
// @param val any 待转换的值
// @return @1 *uint 转换后的 uint 指针
func UintPointer(val any) *uint {
	v := Uint(val)
	return &v
}

// UintsPointer 将任意值转换为 uint 切片指针
// @param val any 待转换的值
// @return @1 *[]uint 转换后的 uint 切片指针
func UintsPointer(val any) *[]uint {
	v := Uints(val)
	return &v
}
