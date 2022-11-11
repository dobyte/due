package xconv

import (
	"reflect"
	"strconv"
	"time"
)

func Float64(any interface{}) float64 {
	if any == nil {
		return 0
	}

	toFloat64 := func(v complex128) float64 {
		s := strconv.FormatComplex(v, 'f', -1, 64)
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}

	switch v := any.(type) {
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
		var (
			rv   = reflect.ValueOf(any)
			kind = rv.Kind()
		)

		for kind == reflect.Ptr {
			rv = rv.Elem()
			kind = rv.Kind()
		}

		switch kind {
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

func Float64s(any interface{}) (slice []float64) {
	if any == nil {
		return
	}

	switch v := any.(type) {
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
	case []interface{}:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = Float64(v[i])
		}
	case *[]interface{}:
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
		var (
			rv   = reflect.ValueOf(any)
			kind = rv.Kind()
		)

		for kind == reflect.Ptr {
			rv = rv.Elem()
			kind = rv.Kind()
		}

		switch kind {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]float64, count)
			for i := 0; i < count; i++ {
				slice[i] = Float64(rv.Index(i).Interface())
			}
		}
	}

	return
}

func Float64Pointer(any interface{}) *float64 {
	v := Float64(any)
	return &v
}

func Float64sPointer(any interface{}) *[]float64 {
	v := Float64s(any)
	return &v
}
