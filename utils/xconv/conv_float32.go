package xconv

import "reflect"

func Float32(any interface{}) float32 {
	return float32(Float64(any))
}

func Float32s(any interface{}) (slice []float32) {
	if any == nil {
		return
	}

	switch v := any.(type) {
	case []int:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]int:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []int8:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]int8:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []int16:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]int16:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []int32:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]int32:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []int64:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]int64:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []uint:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]uint:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []uint8:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]uint8:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []uint16:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]uint16:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []uint32:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]uint32:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []uint64:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]uint64:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []float32:
		return v
	case *[]float32:
		return *v
	case []float64:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]float64:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []complex64:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]complex64:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []complex128:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]complex128:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []string:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]string:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []bool:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]bool:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case []interface{}:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[]interface{}:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
		}
	case [][]byte:
		slice = make([]float32, len(v))
		for i := range v {
			slice[i] = Float32(v[i])
		}
	case *[][]byte:
		slice = make([]float32, len(*v))
		for i := range *v {
			slice[i] = Float32((*v)[i])
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
			slice = make([]float32, count)
			for i := 0; i < count; i++ {
				slice[i] = Float32(rv.Index(i).Interface())
			}
		}
	}

	return
}

func Float32Pointer(any interface{}) *float32 {
	v := Float32(any)
	return &v
}

func Float32sPointer(any interface{}) *[]float32 {
	v := Float32s(any)
	return &v
}
