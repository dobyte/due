package xconv

import "reflect"

func Int(any interface{}) int {
	return int(Int64(any))
}

func Ints(any interface{}) (slice []int) {
	if any == nil {
		return
	}

	switch v := any.(type) {
	case []int:
		return v
	case *[]int:
		return *v
	case []int8:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]int8:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []int16:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]int16:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []int32:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]int32:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []int64:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]int64:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []uint:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]uint:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []uint8:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]uint8:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []uint16:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]uint16:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []uint32:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]uint32:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []uint64:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]uint64:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []float32:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]float32:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []float64:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]float64:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []complex64:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]complex64:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []complex128:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]complex128:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []string:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]string:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []bool:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]bool:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case []interface{}:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]interface{}:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
		}
	case [][]byte:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[][]byte:
		slice = make([]int, len(*v))
		for i := range *v {
			slice[i] = Int((*v)[i])
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
			slice = make([]int, count)
			for i := 0; i < count; i++ {
				slice[i] = Int(rv.Index(i).Interface())
			}
		}
	}

	return
}

func IntPointer(any interface{}) *int {
	v := Int(any)
	return &v
}

func IntsPointer(any interface{}) *[]int {
	v := Ints(any)
	return &v
}
