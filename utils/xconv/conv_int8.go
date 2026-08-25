package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Int8 将任意值转换为 int8
// @param val any 待转换的值
// @return @1 int8 转换后的 int8
func Int8(val any) int8 {
	return int8(Int64(val))
}

// Int8s 将任意值转换为 int8 切片
// @param val any 待转换的值
// @return @1 []int8 转换后的 int8 切片
func Int8s(val any) (slice []int8) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]int:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []int8:
		return v
	case *[]int8:
		return *v
	case []int16:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]int16:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []int32:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]int32:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []int64:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]int64:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []uint:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]uint:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []uint8:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]uint8:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []uint16:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]uint16:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []uint32:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]uint32:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []uint64:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]uint64:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []float32:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]float32:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []float64:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]float64:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []complex64:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]complex64:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []complex128:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]complex128:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []string:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]string:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []bool:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]bool:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []any:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]any:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case [][]byte:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[][]byte:
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]int8, count)
			for i := range count {
				slice[i] = Int8(rv.Index(i).Interface())
			}
		}
	}

	return
}

// Int8Pointer 将任意值转换为 int8 指针
// @param val any 待转换的值
// @return @1 *int8 转换后的 int8 指针
func Int8Pointer(any any) *int8 {
	v := Int8(any)
	return &v
}

// Int8sPointer 将任意值转换为 int8 切片指针
// @param val any 待转换的值
// @return @1 *[]int8 转换后的 int8 切片指针
func Int8sPointer(any any) *[]int8 {
	v := Int8s(any)
	return &v
}
