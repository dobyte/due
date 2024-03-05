package xconv

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"strconv"
	"time"
)

func Int64(any interface{}) int64 {
	if any == nil {
		return 0
	}

	switch v := any.(type) {
	case int:
		return int64(v)
	case *int:
		return int64(*v)
	case int8:
		return int64(v)
	case *int8:
		return int64(*v)
	case int16:
		return int64(v)
	case *int16:
		return int64(*v)
	case int32:
		return int64(v)
	case *int32:
		return int64(*v)
	case int64:
		return v
	case *int64:
		return *v
	case uint:
		return int64(v)
	case *uint:
		return int64(*v)
	case uint8:
		return int64(v)
	case *uint8:
		return int64(*v)
	case uint16:
		return int64(v)
	case *uint16:
		return int64(*v)
	case uint32:
		return int64(v)
	case *uint32:
		return int64(*v)
	case uint64:
		return int64(v)
	case *uint64:
		return int64(*v)
	case float32:
		return int64(v)
	case *float32:
		return int64(*v)
	case float64:
		return int64(v)
	case *float64:
		return int64(*v)
	case complex64:
		return int64(real(v))
	case *complex64:
		return int64(real(*v))
	case complex128:
		return int64(real(v))
	case *complex128:
		return int64(real(*v))
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
		return v.UnixNano()
	case *time.Time:
		return v.UnixNano()
	case []byte:
		buf := make([]byte, 8)
		copy(buf[len(buf)-len(v):], v)

		var i int64
		if err := binary.Read(bytes.NewReader(buf), binary.BigEndian, &i); err == nil {
			return i
		} else {
			return 0
		}
	case *[]byte:
		return Int64(*v)
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
			return Int64(rv.Bool())
		case reflect.String:
			i, _ := strconv.ParseInt(rv.String(), 10, 64)
			return i
		case reflect.Uintptr:
			return int64(rv.Uint())
		case reflect.UnsafePointer:
			return int64(rv.Pointer())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return rv.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return int64(rv.Uint())
		case reflect.Float32, reflect.Float64:
			return int64(rv.Float())
		case reflect.Complex64, reflect.Complex128:
			return int64(real(rv.Complex()))
		default:
			return 0
		}
	}
}

func Int64s(any interface{}) (slice []int64) {
	if any == nil {
		return
	}

	switch v := any.(type) {
	case []int:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]int:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []int8:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]int8:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []int16:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]int16:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []int32:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]int32:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []int64:
		return v
	case *[]int64:
		return *v
	case []uint:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]uint:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []uint8:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]uint8:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []uint16:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]uint16:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []uint32:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]uint32:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []uint64:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]uint64:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []float32:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]float32:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []float64:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]float64:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []complex64:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]complex64:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []complex128:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]complex128:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []string:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]string:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []bool:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]bool:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case []interface{}:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[]interface{}:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
		}
	case [][]byte:
		slice = make([]int64, len(v))
		for i := range v {
			slice[i] = Int64(v[i])
		}
	case *[][]byte:
		slice = make([]int64, len(*v))
		for i := range *v {
			slice[i] = Int64((*v)[i])
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
			slice = make([]int64, count)
			for i := 0; i < count; i++ {
				slice[i] = Int64(rv.Index(i).Interface())
			}
		}
	}

	return
}

func Int64Pointer(any interface{}) *int64 {
	v := Int64(any)
	return &v
}

func Int64sPointer(any interface{}) *[]int64 {
	v := Int64s(any)
	return &v
}
