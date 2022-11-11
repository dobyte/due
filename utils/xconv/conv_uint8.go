package xconv

import "reflect"

func Uint8(any interface{}) uint8 {
	return uint8(Uint64(any))
}

func Uint8s(any interface{}) (slice []uint8) {
	if any == nil {
		return
	}

	switch v := any.(type) {
	case []int:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]int:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []int8:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]int8:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []int16:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]int16:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []int32:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]int32:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []int64:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]int64:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []uint:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]uint:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []uint8:
		return v
	case *[]uint8:
		return *v
	case []uint16:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]uint16:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []uint32:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]uint32:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []uint64:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]uint64:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []float32:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]float32:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []float64:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]float64:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []complex64:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]complex64:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []complex128:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]complex128:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []string:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]string:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []bool:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]bool:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case []interface{}:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]interface{}:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
		}
	case [][]byte:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[][]byte:
		slice = make([]uint8, len(*v))
		for i := range *v {
			slice[i] = Uint8((*v)[i])
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
			slice = make([]uint8, count)
			for i := 0; i < count; i++ {
				slice[i] = Uint8(rv.Index(i).Interface())
			}
		}
	}

	return
}

func Uint8Pointer(any interface{}) *uint8 {
	v := Uint8(any)
	return &v
}

func Uint8sPointer(any interface{}) *[]uint8 {
	v := Uint8s(any)
	return &v
}
