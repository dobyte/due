package xconv

import "reflect"

func Uint16(any interface{}) uint16 {
	return uint16(Uint64(any))
}

func Uint16s(any interface{}) (slice []uint16) {
	if any == nil {
		return
	}

	switch v := any.(type) {
	case []int:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]int:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []int8:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]int8:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []int16:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]int16:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []int32:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]int32:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []int64:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]int64:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []uint:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]uint:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []uint8:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]uint8:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []uint16:
		return v
	case *[]uint16:
		return *v
	case []uint32:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]uint32:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []uint64:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]uint64:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []float32:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]float32:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []float64:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]float64:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []complex64:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]complex64:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []complex128:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]complex128:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []string:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]string:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []bool:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]bool:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case []interface{}:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]interface{}:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
		}
	case [][]byte:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[][]byte:
		slice = make([]uint16, len(*v))
		for i := range *v {
			slice[i] = Uint16((*v)[i])
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
			slice = make([]uint16, count)
			for i := 0; i < count; i++ {
				slice[i] = Uint16(rv.Index(i).Interface())
			}
		}
	}

	return
}

func Uint16Pointer(any interface{}) *uint16 {
	v := Uint16(any)
	return &v
}

func Uint16sPointer(any interface{}) *[]uint16 {
	v := Uint16s(any)
	return &v
}
