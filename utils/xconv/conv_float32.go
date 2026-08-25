package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Float32 将任意值转换为 float32
// @param val any 待转换的值
// @return @1 float32 转换后的 float32
func Float32(val any) float32 {
	return float32(Float64(val))
}

// Float32s 将任意值转换为 float32 切片
// @param val any 待转换的值
// @return @1 []float32 转换后的 float32 切片
func Float32s(val any) (slice []float32) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]int:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []int8:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]int8:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []int16:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]int16:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []int32:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]int32:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []int64:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]int64:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []uint:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]uint:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []uint8:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]uint8:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []uint16:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]uint16:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []uint32:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]uint32:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []uint64:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]uint64:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []float32:
		return v
	case *[]float32:
		return *v
	case []float64:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]float64:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []complex64:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]complex64:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []complex128:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]complex128:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []string:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]string:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []bool:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]bool:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []any:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]any:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case [][]byte:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[][]byte:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]float32, count)
			for i := range count {
				slice[i] = Float32(rv.Index(i).Interface())
			}
		}
	}

	return
}

// Float32Pointer 将任意值转换为 float32 指针
// @param val any 待转换的值
// @return @1 *float32 转换后的 float32 指针
func Float32Pointer(any any) *float32 {
	v := Float32(any)
	return &v
}

// Float32sPointer 将任意值转换为 float32 切片指针
// @param val any 待转换的值
// @return @1 *[]float32 转换后的 float32 切片指针
func Float32sPointer(any any) *[]float32 {
	v := Float32s(any)
	return &v
}
