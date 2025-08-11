package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

func Uint(val any) uint {
	return uint(Uint64(val))
}

func Uints(val any) (slice []uint) {
	if val == nil {
		return
	}

	switch v := val.(type) {
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
	case []any:
		slice = make([]uint, len(v))
		for i := range v {
			slice[i] = Uint(v[i])
		}
	case *[]any:
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
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]uint, count)
			for i := range count {
				slice[i] = Uint(rv.Index(i).Interface())
			}
		}
	}

	return
}

func UintPointer(val any) *uint {
	v := Uint(val)
	return &v
}

func UintsPointer(val any) *[]uint {
	v := Uints(val)
	return &v
}
