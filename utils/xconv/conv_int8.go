package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Int8 converts val to an int8.
func Int8(val any) int8 {
	return int8(Int64(val))
}

// Int8s converts val to an int8 slice.
func Int8s(val any) (slice []int8) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]int:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []int8:
		return v
	case *[]int8:
		if v == nil {
			return
		}
		return *v
	case []int16:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]int16:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []int32:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]int32:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []int64:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]int64:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []uint:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]uint:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []uint8:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]uint8:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []uint16:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]uint16:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []uint32:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]uint32:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []uint64:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]uint64:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []float32:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]float32:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []float64:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]float64:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []complex64:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]complex64:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []complex128:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]complex128:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []string:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]string:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []bool:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]bool:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case []any:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[]any:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	case [][]byte:
		slice = make([]int8, len(v))
		for i := range v {
			slice[i] = Int8(v[i])
		}
	case *[][]byte:
		if v == nil {
			return
		}
		slice = make([]int8, len(*v))
		for i := range *v {
			slice[i] = Int8((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]int8, count)
			for i := range count {
				slice[i] = Int8(rv.Index(i).Interface())
			}
		}
	}

	return
}

// Int8Pointer converts val to a pointer to int8.
func Int8Pointer(any any) *int8 {
	v := Int8(any)
	return &v
}

// Int8sPointer converts val to a pointer to an int8 slice.
func Int8sPointer(any any) *[]int8 {
	v := Int8s(any)
	return &v
}
