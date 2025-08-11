package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

func Int16(val any) int16 {
	return int16(Int64(val))
}

func Int16s(val any) (slice []int16) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]int:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []int8:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]int8:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []int16:
		return v
	case *[]int16:
		return *v
	case []int32:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]int32:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []int64:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]int64:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []uint:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]uint:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []uint8:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]uint8:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []uint16:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]uint16:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []uint32:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]uint32:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []uint64:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]uint64:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []float32:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]float32:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []float64:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]float64:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []complex64:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]complex64:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []complex128:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]complex128:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []string:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]string:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []bool:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]bool:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case []any:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[]any:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	case [][]byte:
		slice = make([]int16, len(v))
		for i := range v {
			slice[i] = Int16(v[i])
		}
	case *[][]byte:
		slice = make([]int16, len(*v))
		for i := range *v {
			slice[i] = Int16((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]int16, count)
			for i := range count {
				slice[i] = Int16(rv.Index(i).Interface())
			}
		}
	}

	return
}

func Int16Pointer(any any) *int16 {
	v := Int16(any)
	return &v
}

func Int16sPointer(any any) *[]int16 {
	v := Int16s(any)
	return &v
}
