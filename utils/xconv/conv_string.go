package xconv

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
	"unsafe"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/utils/xreflect"
)

func String(val any) string {
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case int:
		return strconv.Itoa(v)
	case *int:
		return strconv.Itoa(*v)
	case int8:
		return strconv.Itoa(int(v))
	case *int8:
		return strconv.Itoa(int(*v))
	case int16:
		return strconv.Itoa(int(v))
	case *int16:
		return strconv.Itoa(int(*v))
	case int32:
		return strconv.Itoa(int(v))
	case *int32:
		return strconv.Itoa(int(*v))
	case int64:
		return strconv.FormatInt(v, 10)
	case *int64:
		return strconv.FormatInt(*v, 10)
	case uint:
		return strconv.Itoa(int(v))
	case *uint:
		return strconv.Itoa(int(*v))
	case uint8:
		return strconv.Itoa(int(v))
	case *uint8:
		return strconv.Itoa(int(*v))
	case uint16:
		return strconv.Itoa(int(v))
	case *uint16:
		return strconv.Itoa(int(*v))
	case uint32:
		return strconv.Itoa(int(v))
	case *uint32:
		return strconv.Itoa(int(*v))
	case uint64:
		return strconv.FormatInt(int64(v), 10)
	case *uint64:
		return strconv.FormatInt(int64(*v), 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case *float32:
		return strconv.FormatFloat(float64(*v), 'f', -1, 64)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case *float64:
		return strconv.FormatFloat(*v, 'f', -1, 64)
	case complex64:
		return strconv.FormatComplex(complex128(v), 'e', -1, 64)
	case complex128:
		return strconv.FormatComplex(v, 'e', -1, 128)
	case *complex64:
		return strconv.FormatComplex(complex128(*v), 'e', -1, 64)
	case *complex128:
		return strconv.FormatComplex(*v, 'e', -1, 128)
	case bool:
		return strconv.FormatBool(v)
	case *bool:
		return strconv.FormatBool(*v)
	case string:
		return v
	case *string:
		return *v
	case []byte:
		return BytesToString(v)
	case *[]byte:
		return BytesToString(*v)
	case time.Time:
		if v.IsZero() {
			return ""
		}
		return v.String()
	case *time.Time:
		if v.IsZero() {
			return ""
		}
		return v.String()
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Invalid:
			return ""
		case reflect.Bool:
			return String(rv.Bool())
		case reflect.String:
			return rv.String()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return String(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return String(rv.Uint())
		case reflect.Float32, reflect.Float64:
			return String(rv.Float())
		case reflect.Complex64, reflect.Complex128:
			return String(rv.Complex())
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return fmt.Sprintf("%v", v)
			}
			return *(*string)(unsafe.Pointer(&b))
		}
	}
}

func Strings(val any) (slice []string) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]int:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []int8:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]int8:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []int16:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]int16:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []int32:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]int32:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []int64:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]int64:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []uint:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]uint:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []uint8:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]uint8:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []uint16:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]uint16:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []uint32:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]uint32:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []uint64:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]uint64:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []float32:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]float32:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []float64:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]float64:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []complex64:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]complex64:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []complex128:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]complex128:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []string:
		return v
	case *[]string:
		return *v
	case []bool:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]bool:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case []any:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]any:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	case [][]byte:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[][]byte:
		slice = make([]string, len(*v))
		for i := range *v {
			slice[i] = String((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]string, count)
			for i := range count {
				slice[i] = String(rv.Index(i).Interface())
			}
		}
	}

	return
}

func StringPointer(val any) *string {
	v := String(val)
	return &v
}

func StringsPointer(val any) *[]string {
	v := Strings(val)
	return &v
}
