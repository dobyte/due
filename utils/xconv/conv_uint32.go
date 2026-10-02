package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Uint32 converts val to a uint32.
func Uint32(val any) uint32 {
	return uint32(Uint64(val))
}

// Uint32s converts val to a uint32 slice.
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
		slice = make([]uint32, len(*v))
		for i := range *v {
			slice[i] = Uint32((*v)[i])
		}
	case []uint32:
		return v
	case *[]uint32:
		if v == nil {
			return
		}
		return *v
	case []uint64:
		slice = make([]uint32, len(v))
		for i := range v {
			slice[i] = Uint32(v[i])
		}
	case *[]uint64:
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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
		if v == nil {
			return
		}
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

// Uint32Pointer converts val to a pointer to uint32.
func Uint32Pointer(val any) *uint32 {
	v := Uint32(val)
	return &v
}

// Uint32sPointer converts val to a pointer to a uint32 slice.
func Uint32sPointer(val any) *[]uint32 {
	v := Uint32s(val)
	return &v
}
