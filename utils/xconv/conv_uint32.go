package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

func Uint32(val any) uint32 {
	return uint32(Uint64(val))
}

func Uint32s(val any) (slice []uint32) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]int:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []int8:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]int8:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []int16:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]int16:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []int32:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]int32:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []int64:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]int64:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []uint:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]uint:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []uint8:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]uint8:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []uint16:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]uint16:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []uint32:
		return v
	case *[]uint32:
		return *v
	case []uint64:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]uint64:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []float32:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]float32:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []float64:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]float64:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []complex64:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]complex64:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []complex128:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]complex128:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []string:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]string:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []bool:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]bool:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []any:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]any:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case [][]byte:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[][]byte:
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]uint32, count)
			for i := range count {
				slice[i] = Uint32(rv.Index(i).Interface())
			}
		}
	}

	return
}

func Uint32Pointer(val any) *uint32 {
	v := Uint32(val)
	return &v
}

func Uint32sPointer(val any) *[]uint32 {
	v := Uint32s(val)
	return &v
}
