package xconv

import (
	"reflect"
	"strings"
	"time"
	"unsafe"
)

func Bool(any interface{}) bool {
	if any == nil {
		return false
	}

	toBool := func(v string) bool {
		return v != "" && v != "0" && strings.ToLower(v) != "false"
	}

	switch v := any.(type) {
	case int:
		return v != 0
	case *int:
		return *v != 0
	case int8:
		return v != 0
	case *int8:
		return *v != 0
	case int16:
		return v != 0
	case *int16:
		return *v != 0
	case int32:
		return v != 0
	case *int32:
		return *v != 0
	case int64:
		return v != 0
	case *int64:
		return *v != 0
	case uint:
		return v != 0
	case *uint:
		return *v != 0
	case uint8:
		return v != 0
	case *uint8:
		return *v != 0
	case uint16:
		return v != 0
	case *uint16:
		return *v != 0
	case uint32:
		return v != 0
	case *uint32:
		return *v != 0
	case uint64:
		return v != 0
	case *uint64:
		return *v != 0
	case float32:
		return v != 0
	case *float32:
		return *v != 0
	case float64:
		return v != 0
	case *float64:
		return *v != 0
	case complex64:
		return toBool(String(v))
	case *complex64:
		return toBool(String(*v))
	case complex128:
		return toBool(String(v))
	case *complex128:
		return toBool(String(*v))
	case bool:
		return v
	case *bool:
		return *v
	case string:
		return toBool(v)
	case *string:
		return toBool(*v)
	case []byte:
		return toBool(*(*string)(unsafe.Pointer(&v)))
	case *[]byte:
		return toBool(*(*string)(unsafe.Pointer(v)))
	case time.Time:
		return v.IsZero()
	case *time.Time:
		return v.IsZero()
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
		case reflect.Bool:
			return rv.Bool()
		case reflect.String:
			return toBool(rv.String())
		case reflect.Uintptr:
			return rv.Uint() != 0
		case reflect.UnsafePointer:
			return !rv.IsNil() && uint(rv.Pointer()) != 0
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return rv.Int() != 0
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return rv.Uint() != 0
		case reflect.Float32, reflect.Float64:
			return rv.Float() != 0
		case reflect.Complex64, reflect.Complex128:
			return toBool(String(rv.Complex()))
		case reflect.Array:
			return rv.Len() != 0
		case reflect.Slice, reflect.Map:
			return !rv.IsNil() && rv.Len() != 0
		case reflect.Struct:
			return true
		case reflect.Chan, reflect.Func, reflect.Interface:
			return !rv.IsNil()
		default:
			return false
		}
	}
}

func Bools(any interface{}) (slice []bool) {
	if any == nil {
		return
	}

	switch v := any.(type) {
	case []int:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]int:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []int8:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]int8:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []int16:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]int16:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []int32:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]int32:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []int64:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]int64:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []uint:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]uint:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []uint8:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]uint8:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []uint16:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]uint16:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []uint32:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]uint32:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []uint64:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]uint64:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []float32:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]float32:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []float64:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]float64:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []complex64:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]complex64:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []complex128:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]complex128:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []string:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]string:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []bool:
		return v
	case *[]bool:
		return *v
	case []interface{}:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]interface{}:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case [][]byte:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[][]byte:
		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
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
			slice = make([]bool, count)
			for i := 0; i < count; i++ {
				slice[i] = Bool(rv.Index(i).Interface())
			}
		}
	}

	return
}

func BoolPointer(any interface{}) *bool {
	v := Bool(any)
	return &v
}

func BoolsPointer(any interface{}) *[]bool {
	v := Bools(any)
	return &v
}
