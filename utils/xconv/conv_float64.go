package xconv

import (
	"reflect"
	"strconv"
	"time"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Float64 将任意值转换为 float64
// 支持所有基础数值类型（复数取实部）、bool、time.Time（Unix纳秒）及
// 通过反射处理的字符串等类型；无法转换时返回 0
// @param val any 待转换的值
// @return @1 float64 转换后的 float64
func Float64(val any) float64 {
	if val == nil {
		return 0
	}

	toFloat64 := func(v complex128) float64 {
		return real(v)
	}

	switch v := val.(type) {
	case int:
		return float64(v)
	case *int:
		return float64(*v)
	case int8:
		return float64(v)
	case *int8:
		return float64(*v)
	case int16:
		return float64(v)
	case *int16:
		return float64(*v)
	case int32:
		return float64(v)
	case *int32:
		return float64(*v)
	case int64:
		return float64(v)
	case *int64:
		return float64(*v)
	case uint:
		return float64(v)
	case *uint:
		return float64(*v)
	case uint8:
		return float64(v)
	case *uint8:
		return float64(*v)
	case uint16:
		return float64(v)
	case *uint16:
		return float64(*v)
	case uint32:
		return float64(v)
	case *uint32:
		return float64(*v)
	case uint64:
		return float64(v)
	case *uint64:
		return float64(*v)
	case float32:
		return float64(v)
	case *float32:
		return float64(*v)
	case float64:
		return v
	case *float64:
		return *v
	case complex64:
		return toFloat64(complex128(v))
	case *complex64:
		return toFloat64(complex128(*v))
	case complex128:
		return toFloat64(v)
	case *complex128:
		return toFloat64(*v)
	case bool:
		if v {
			return 1
		}
		return 0
	case *bool:
		if *v {
			return 1
		}
		return 0
	case time.Time:
		return float64(v.UnixNano())
	case *time.Time:
		return float64(v.UnixNano())
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Bool:
			return Float64(rv.Bool())
		case reflect.String:
			i, _ := strconv.ParseFloat(rv.String(), 64)
			return i
		case reflect.Uintptr:
			return float64(rv.Uint())
		case reflect.UnsafePointer:
			return float64(rv.Pointer())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return float64(rv.Uint())
		case reflect.Float32, reflect.Float64:
			return rv.Float()
		case reflect.Complex64, reflect.Complex128:
			return toFloat64(rv.Complex())
		default:
			return 0
		}
	}
}

// Float64s 将任意值转换为 float64 切片
// @param val any 待转换的值
// @return @1 []float64 转换后的 float64 切片
func Float64s(val any) (slice []float64) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]int:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []int8:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]int8:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []int16:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]int16:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []int32:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]int32:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []int64:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]int64:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []uint:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]uint:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []uint8:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]uint8:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []uint16:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]uint16:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []uint32:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]uint32:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []uint64:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]uint64:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []float32:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]float32:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []float64:
		return v
	case *[]float64:
		return *v
	case []complex64:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]complex64:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []complex128:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]complex128:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []string:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]string:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []bool:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]bool:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []any:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]any:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case [][]byte:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[][]byte:
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]float64, count)
			for i := range count {
				slice[i] = Float64(rv.Index(i).Interface())
			}
		}
	}

	return
}

// Float64Pointer 将任意值转换为 float64 指针
// @param val any 待转换的值
// @return @1 *float64 转换后的 float64 指针
func Float64Pointer(any any) *float64 {
	v := Float64(any)
	return &v
}

// Float64sPointer 将任意值转换为 float64 切片指针
// @param val any 待转换的值
// @return @1 *[]float64 转换后的 float64 切片指针
func Float64sPointer(any any) *[]float64 {
	v := Float64s(any)
	return &v
}
