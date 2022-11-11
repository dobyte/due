package xconv

import "reflect"

func Int8(any interface{}) int8 {
	return int8(Int64(any))
}

func Int8s(any interface{}) (slice []int8) {
	if any == nil {
		return
	}

	switch v := any.(type) {
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
	case []interface{}:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]interface{}:
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
			slice = make([]int8, count)
			for i := 0; i < count; i++ {
				slice[i] = Int8(rv.Index(i).Interface())
			}
		}
	}

	return
}

func Int8Pointer(any interface{}) *int8 {
	v := Int8(any)
	return &v
}

func Int8sPointer(any interface{}) *[]int8 {
	v := Int8s(any)
	return &v
}
