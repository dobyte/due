package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

func Int(val any) int {
	return int(Int64(val))
}

func Ints(val any) (slice []int) {
	if val == nil {
		return
	}

	switch v := val.(type) {
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
	case []any:
		slice = make([]int, len(v))
		for i := range v {
			slice[i] = Int(v[i])
		}
	case *[]any:
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
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]int, count)
			for i := range count {
				slice[i] = Int(rv.Index(i).Interface())
			}
		}
	}

	return
}

func IntPointer(any any) *int {
	v := Int(any)
	return &v
}

func IntsPointer(any any) *[]int {
	v := Ints(any)
	return &v
}
