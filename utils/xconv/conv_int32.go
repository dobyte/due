package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Int32 将任意值转换为 int32
// @param val any 待转换的值
// @return @1 int32 转换后的 int32
func Int32(val any) int32 {
	return int32(Int64(val))
}

// Int32s 将任意值转换为 int32 切片
// @param val any 待转换的值
// @return @1 []int32 转换后的 int32 切片
func Int32s(val any) (slice []int32) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]int:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []int8:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]int8:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []int16:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]int16:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []int32:
		return v
	case *[]int32:
		return *v
	case []int64:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]int64:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []uint:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]uint:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []uint8:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]uint8:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []uint16:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]uint16:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []uint32:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]uint32:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []uint64:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]uint64:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []float32:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]float32:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []float64:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]float64:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []complex64:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]complex64:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []complex128:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]complex128:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []string:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]string:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []bool:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]bool:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case []any:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[]any:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	case [][]byte:
		slice = make([]int32, len(v))
		for i := range v {
			slice[i] = Int32(v[i])
		}
	case *[][]byte:
		slice = make([]int32, len(*v))
		for i := range *v {
			slice[i] = Int32((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]int32, count)
			for i := range count {
				slice[i] = Int32(rv.Index(i).Interface())
			}
		}
	}

	return
}

// Int32Pointer 将任意值转换为 int32 指针
// @param val any 待转换的值
// @return @1 *int32 转换后的 int32 指针
func Int32Pointer(val any) *int32 {
	v := Int32(val)
	return &v
}

// Int32sPointer 将任意值转换为 int32 切片指针
// @param val any 待转换的值
// @return @1 *[]int32 转换后的 int32 切片指针
func Int32sPointer(val any) *[]int32 {
	v := Int32s(val)
	return &v
}
