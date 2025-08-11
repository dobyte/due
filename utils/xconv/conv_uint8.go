package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

func Uint8(val any) uint8 {
	return uint8(Uint64(val))
}

func Uint8s(val any) (slice []uint8) {
	if val == nil {
		return
	}

	switch v := val.(type) {
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
	case []any:
		slice = make([]uint8, len(v))
		for i := range v {
			slice[i] = Uint8(v[i])
		}
	case *[]any:
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
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]uint8, count)
			for i := range count {
				slice[i] = Uint8(rv.Index(i).Interface())
			}
		}
	}

	return
}

func Uint8Pointer(val any) *uint8 {
	v := Uint8(val)
	return &v
}

func Uint8sPointer(val any) *[]uint8 {
	v := Uint8s(val)
	return &v
}
