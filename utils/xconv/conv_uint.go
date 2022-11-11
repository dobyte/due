package xconv

import "reflect"

func Uint(any interface{}) uint {
	return uint(Uint64(any))
}

// Uints 任何类型转uint切片
func Uints(any interface{}) (slice []uint) {
	if any == nil {
		return
	}

	switch v := any.(type) {
	case []int:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]int:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []int8:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]int8:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []int16:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]int16:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []int32:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]int32:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []int64:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]int64:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []uint:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]uint:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []uint8:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]uint8:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []uint16:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]uint16:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []uint32:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]uint32:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []uint64:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]uint64:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []float32:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]float32:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []float64:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]float64:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []complex64:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]complex64:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []complex128:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]complex128:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []string:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]string:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []bool:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]bool:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case []interface{}:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]interface{}:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
		}
	case [][]byte:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[][]byte:
		slice = make([]uint, len(*v))
		for i := range *v {
			slice[i] = Uint((*v)[i])
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
			slice = make([]uint, count)
			for i := 0; i < count; i++ {
				slice[i] = Uint(rv.Index(i).Interface())
			}
		}
	}

	return
}

func UintPointer(any interface{}) *uint {
	v := Uint(any)
	return &v
}

func UintsPointer(any interface{}) *[]uint {
	v := Uints(any)
	return &v
}
