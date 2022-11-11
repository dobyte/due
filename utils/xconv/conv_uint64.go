package xconv

import (
	"reflect"
	"strconv"
	"time"
)

func Uint64(any interface{}) uint64 {
	if any == nil {
		return 0
	}

	switch v := any.(type) {
	case int:
		return uint64(v)
	case *int:
		return uint64(*v)
	case int8:
		return uint64(v)
	case *int8:
		return uint64(*v)
	case int16:
		return uint64(v)
	case *int16:
		return uint64(*v)
	case int32:
		return uint64(v)
	case *int32:
		return uint64(*v)
	case int64:
		return uint64(v)
	case *int64:
		return uint64(*v)
	case uint:
		return uint64(v)
	case *uint:
		return uint64(*v)
	case uint8:
		return uint64(v)
	case *uint8:
		return uint64(*v)
	case uint16:
		return uint64(v)
	case *uint16:
		return uint64(*v)
	case uint32:
		return uint64(v)
	case *uint32:
		return uint64(*v)
	case uint64:
		return v
	case *uint64:
		return *v
	case float32:
		return uint64(v)
	case *float32:
		return uint64(*v)
	case float64:
		return uint64(v)
	case *float64:
		return uint64(*v)
	case complex64:
		return uint64(real(v))
	case *complex64:
		return uint64(real(*v))
	case complex128:
		return uint64(real(v))
	case *complex128:
		return uint64(real(*v))
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
		return uint64(v.UnixNano())
	case *time.Time:
		return uint64(v.UnixNano())
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
			return Uint64(rv.Bool())
		case reflect.String:
			i, _ := strconv.ParseUint(rv.String(), 0, 64)
			return i
		case reflect.Uintptr:
			return rv.Uint()
		case reflect.UnsafePointer:
			return uint64(rv.Pointer())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return Uint64(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return rv.Uint()
		case reflect.Float32, reflect.Float64:
			return uint64(rv.Float())
		case reflect.Complex64, reflect.Complex128:
			return uint64(real(rv.Complex()))
		default:
			return 0
		}
	}
}

func Uint64s(any interface{}) (slice []uint64) {
	if any == nil {
		return
	}

	switch v := any.(type) {
	case []int:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]int:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []int8:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]int8:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []int16:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]int16:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []int32:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]int32:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []int64:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]int64:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []uint:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]uint:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []uint8:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]uint8:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []uint16:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]uint16:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []uint32:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]uint32:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []uint64:
		return v
	case *[]uint64:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []float32:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]float32:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []float64:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]float64:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []complex64:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]complex64:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []complex128:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]complex128:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []string:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]string:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []bool:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]bool:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case []interface{}:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[]interface{}:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
		}
	case [][]byte:
		slice = make([]uint64, len(v))
		for i := range v {
			slice[i] = Uint64(v[i])
		}
	case *[][]byte:
		slice = make([]uint64, len(*v))
		for i := range *v {
			slice[i] = Uint64((*v)[i])
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
			slice = make([]uint64, count)
			for i := 0; i < count; i++ {
				slice[i] = Uint64(rv.Index(i).Interface())
			}
		}
	}

	return
}

func Uint64Pointer(any interface{}) *uint64 {
	v := Uint64(any)
	return &v
}

func Uint64sPointer(any interface{}) *[]uint64 {
	v := Uint64s(any)
	return &v
}
