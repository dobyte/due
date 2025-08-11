package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

func Uint16(val any) uint16 {
	return uint16(Uint64(val))
}

func Uint16s(val any) (slice []uint16) {
	if val == nil {
		return
	}

	switch v := val.(type) {
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
	case []any:
		slice = make([]uint16, len(v))
		for i := range v {
			slice[i] = Uint16(v[i])
		}
	case *[]any:
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
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]uint16, count)
			for i := range count {
				slice[i] = Uint16(rv.Index(i).Interface())
			}
		}
	}

	return
}

func Uint16Pointer(val any) *uint16 {
	v := Uint16(val)
	return &v
}

func Uint16sPointer(val any) *[]uint16 {
	v := Uint16s(val)
	return &v
}
