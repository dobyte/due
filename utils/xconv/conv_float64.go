package xconv

import (
	"reflect"
	"strconv"
	"time"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Float64 converts val to a float64.
//
// It supports all basic numeric types (the real part is taken for complex numbers), bool,
// time.Time (as Unix nanoseconds) and types handled through reflection such as strings; it
// returns 0 when the conversion fails.
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
		if v == nil {
			return 0
		}
		return float64(*v)
	case int8:
		return float64(v)
	case *int8:
		if v == nil {
			return 0
		}
		return float64(*v)
	case int16:
		return float64(v)
	case *int16:
		if v == nil {
			return 0
		}
		return float64(*v)
	case int32:
		return float64(v)
	case *int32:
		if v == nil {
			return 0
		}
		return float64(*v)
	case int64:
		return float64(v)
	case *int64:
		if v == nil {
			return 0
		}
		return float64(*v)
	case uint:
		return float64(v)
	case *uint:
		if v == nil {
			return 0
		}
		return float64(*v)
	case uint8:
		return float64(v)
	case *uint8:
		if v == nil {
			return 0
		}
		return float64(*v)
	case uint16:
		return float64(v)
	case *uint16:
		if v == nil {
			return 0
		}
		return float64(*v)
	case uint32:
		return float64(v)
	case *uint32:
		if v == nil {
			return 0
		}
		return float64(*v)
	case uint64:
		return float64(v)
	case *uint64:
		if v == nil {
			return 0
		}
		return float64(*v)
	case float32:
		return float64(v)
	case *float32:
		if v == nil {
			return 0
		}
		return float64(*v)
	case float64:
		return v
	case *float64:
		if v == nil {
			return 0
		}
		return *v
	case complex64:
		return toFloat64(complex128(v))
	case *complex64:
		if v == nil {
			return 0
		}
		return toFloat64(complex128(*v))
	case complex128:
		return toFloat64(v)
	case *complex128:
		if v == nil {
			return 0
		}
		return toFloat64(*v)
	case bool:
		if v {
			return 1
		} else {
			return 0
		}
	case *bool:
		if v != nil && *v {
			return 1
		} else {
			return 0
		}
	case time.Time:
		return float64(v.UnixNano())
	case *time.Time:
		if v == nil {
			return 0
		}
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

// Float64s converts val to a float64 slice.
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = Float64((*v)[i])
		}
	case []float64:
		return v
	case *[]float64:
		if v == nil {
			return
		}
		return *v
	case []complex64:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]complex64:
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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

// Float64Pointer converts val to a pointer to float64.
func Float64Pointer(any any) *float64 {
	v := Float64(any)
	return &v
}

// Float64sPointer converts val to a pointer to a float64 slice.
func Float64sPointer(any any) *[]float64 {
	v := Float64s(any)
	return &v
}
