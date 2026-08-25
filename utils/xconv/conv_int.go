package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Int 将任意值转换为 int
// @param val any 待转换的值
// @return @1 int 转换后的 int
func Int(val any) int {
	return int(Int64(val))
}

// Ints 将任意值转换为 int 切片
// @param val any 待转换的值
// @return @1 []int 转换后的 int 切片
func Ints(val any) (slice []int) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		return v
	case *[]int:
		if v == nil {
			return
		}
		return *v
	case []int8:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]int8:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []int16:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]int16:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []int32:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]int32:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []int64:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]int64:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []uint:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]uint:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []uint8:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]uint8:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []uint16:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]uint16:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []uint32:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]uint32:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []uint64:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]uint64:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []float32:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]float32:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []float64:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]float64:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []complex64:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]complex64:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []complex128:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]complex128:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []string:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]string:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []bool:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]bool:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []any:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]any:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case [][]byte:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[][]byte:
		if v == nil {
			return
		}
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]int, count)
			for i := range count {
				slice[i] = Int(rv.Index(i).Interface())
			}
		}
	}

	return
}

// IntPointer 将任意值转换为 int 指针
// @param val any 待转换的值
// @return @1 *int 转换后的 int 指针
func IntPointer(any any) *int {
	v := Int(any)
	return &v
}

// IntsPointer 将任意值转换为 int 切片指针
// @param val any 待转换的值
// @return @1 *[]int 转换后的 int 切片指针
func IntsPointer(any any) *[]int {
	v := Ints(any)
	return &v
}
