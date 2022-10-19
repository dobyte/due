package xconv

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

func Int(any interface{}) int {
	return int(Int64(any))
}

func Int8(any interface{}) int8 {
	return int8(Int64(any))
}

func Int16(any interface{}) int16 {
	return int16(Int64(any))
}

func Int32(any interface{}) int32 {
	return int32(Int64(any))
}

func Int64(any interface{}) int64 {
	if any == nil {
		return 0
	}

	switch v := any.(type) {
	case int:
		return int64(v)
	case *int:
		return int64(*v)
	case int8:
		return int64(v)
	case *int8:
		return int64(*v)
	case int16:
		return int64(v)
	case *int16:
		return int64(*v)
	case int32:
		return int64(v)
	case *int32:
		return int64(*v)
	case int64:
		return v
	case *int64:
		return *v
	case uint:
		return int64(v)
	case *uint:
		return int64(*v)
	case uint8:
		return int64(v)
	case *uint8:
		return int64(*v)
	case uint16:
		return int64(v)
	case *uint16:
		return int64(*v)
	case uint32:
		return int64(v)
	case *uint32:
		return int64(*v)
	case uint64:
		return int64(v)
	case *uint64:
		return int64(*v)
	case float32:
		return int64(v)
	case *float32:
		return int64(*v)
	case float64:
		return int64(v)
	case *float64:
		return int64(*v)
	case complex64:
		return int64(real(v))
	case *complex64:
		return int64(real(*v))
	case complex128:
		return int64(real(v))
	case *complex128:
		return int64(real(*v))
	case bool:
		if v {
			return 1
		}
		return 0
	case *bool:
		if *v {
			return 1
		}
		return 0
	case time.Time:
		return v.UnixNano()
	case *time.Time:
		return v.UnixNano()
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
			return Int64(rv.Bool())
		case reflect.String:
			i, _ := strconv.ParseInt(rv.String(), 0, 64)
			return i
		case reflect.Uintptr:
			return int64(rv.Uint())
		case reflect.UnsafePointer:
			return int64(rv.Pointer())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return rv.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return int64(rv.Uint())
		case reflect.Float32, reflect.Float64:
			return int64(rv.Float())
		case reflect.Complex64, reflect.Complex128:
			return int64(real(rv.Complex()))
		default:
			return 0
		}
	}
}

func Uint(any interface{}) uint {
	return uint(Uint64(any))
}

func Uint8(any interface{}) uint8 {
	return uint8(Uint64(any))
}

func Uint16(any interface{}) uint16 {
	return uint16(Uint64(any))
}

func Uint32(any interface{}) uint32 {
	return uint32(Uint64(any))
}

func Uint64(any interface{}) uint64 {
	if any == nil {
		return 0
	}

	switch v := any.(type) {
	case int:
		return uint64(v)
	case *int:
		return uint64(*v)
	case int8:
		return uint64(v)
	case *int8:
		return uint64(*v)
	case int16:
		return uint64(v)
	case *int16:
		return uint64(*v)
	case int32:
		return uint64(v)
	case *int32:
		return uint64(*v)
	case int64:
		return uint64(v)
	case *int64:
		return uint64(*v)
	case uint:
		return uint64(v)
	case *uint:
		return uint64(*v)
	case uint8:
		return uint64(v)
	case *uint8:
		return uint64(*v)
	case uint16:
		return uint64(v)
	case *uint16:
		return uint64(*v)
	case uint32:
		return uint64(v)
	case *uint32:
		return uint64(*v)
	case uint64:
		return v
	case *uint64:
		return *v
	case float32:
		return uint64(v)
	case *float32:
		return uint64(*v)
	case float64:
		return uint64(v)
	case *float64:
		return uint64(*v)
	case complex64:
		return uint64(real(v))
	case *complex64:
		return uint64(real(*v))
	case complex128:
		return uint64(real(v))
	case *complex128:
		return uint64(real(*v))
	case bool:
		if v {
			return 1
		}
		return 0
	case *bool:
		if *v {
			return 1
		}
		return 0
	case time.Time:
		return uint64(v.UnixNano())
	case *time.Time:
		return uint64(v.UnixNano())
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
			return Uint64(rv.Bool())
		case reflect.String:
			i, _ := strconv.ParseUint(rv.String(), 0, 64)
			return i
		case reflect.Uintptr:
			return rv.Uint()
		case reflect.UnsafePointer:
			return uint64(rv.Pointer())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return Uint64(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return rv.Uint()
		case reflect.Float32, reflect.Float64:
			return uint64(rv.Float())
		case reflect.Complex64, reflect.Complex128:
			return uint64(real(rv.Complex()))
		default:
			return 0
		}
	}
}

func Float32(any interface{}) float32 {
	return float32(Float64(any))
}

func Float64(any interface{}) float64 {
	if any == nil {
		return 0
	}

	toFloat64 := func(v complex128) float64 {
		s := strconv.FormatComplex(v, 'f', -1, 64)
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}

	switch v := any.(type) {
	case int:
		return float64(v)
	case *int:
		return float64(*v)
	case int8:
		return float64(v)
	case *int8:
		return float64(*v)
	case int16:
		return float64(v)
	case *int16:
		return float64(*v)
	case int32:
		return float64(v)
	case *int32:
		return float64(*v)
	case int64:
		return float64(v)
	case *int64:
		return float64(*v)
	case uint:
		return float64(v)
	case *uint:
		return float64(*v)
	case uint8:
		return float64(v)
	case *uint8:
		return float64(*v)
	case uint16:
		return float64(v)
	case *uint16:
		return float64(*v)
	case uint32:
		return float64(v)
	case *uint32:
		return float64(*v)
	case uint64:
		return float64(v)
	case *uint64:
		return float64(*v)
	case float32:
		return float64(v)
	case *float32:
		return float64(*v)
	case float64:
		return v
	case *float64:
		return *v
	case complex64:
		return toFloat64(complex128(v))
	case *complex64:
		return toFloat64(complex128(*v))
	case complex128:
		return toFloat64(v)
	case *complex128:
		return toFloat64(*v)
	case bool:
		if v {
			return 1
		}
		return 0
	case *bool:
		if *v {
			return 1
		}
		return 0
	case time.Time:
		return float64(v.UnixNano())
	case *time.Time:
		return float64(v.UnixNano())
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
			return Float64(rv.Bool())
		case reflect.String:
			i, _ := strconv.ParseFloat(rv.String(), 64)
			return i
		case reflect.Uintptr:
			return float64(rv.Uint())
		case reflect.UnsafePointer:
			return float64(rv.Pointer())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return float64(rv.Uint())
		case reflect.Float32, reflect.Float64:
			return rv.Float()
		case reflect.Complex64, reflect.Complex128:
			return toFloat64(rv.Complex())
		default:
			return 0
		}
	}
}

func Byte(any interface{}) byte {
	return Uint8(any)
}

func Bytes(any interface{}) []byte {
	if any == nil {
		return nil
	}

	switch v := any.(type) {
	case string:
		return []byte(v)
	case *string:
		return []byte(*v)
	case []byte:
		return v
	case *[]byte:
		return *v
	default:
		return nil
	}
}

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

func String(any interface{}) string {
	if any == nil {
		return ""
	}

	switch v := any.(type) {
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
		return *(*string)(unsafe.Pointer(&v))
	case *[]byte:
		return *(*string)(unsafe.Pointer(v))
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
		var (
			rv   = reflect.ValueOf(any)
			kind = rv.Kind()
		)

		for kind == reflect.Ptr {
			rv = rv.Elem()
			kind = rv.Kind()
		}

		switch kind {
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

func Duration(any interface{}) time.Duration {
	if any == nil {
		return 0
	}

	// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h", "d".
	toDuration := func(s string) time.Duration {
		reg := regexp.MustCompile(`(((-?\d+)(\.\d+)?)(d))`)
		d, _ := time.ParseDuration(reg.ReplaceAllStringFunc(strings.ToLower(s), func(ss string) string {
			v, err := strconv.ParseFloat(strings.TrimRight(ss, "d"), 64)
			if err != nil {
				return ""
			}
			return fmt.Sprintf("%dns", int64(v*24*3600*1000*1000*1000))
		}))
		return d
	}

	switch v := any.(type) {
	case int:
		return time.Duration(v)
	case *int:
		return time.Duration(*v)
	case int8:
		return time.Duration(v)
	case *int8:
		return time.Duration(*v)
	case int16:
		return time.Duration(v)
	case *int16:
		return time.Duration(*v)
	case int32:
		return time.Duration(v)
	case *int32:
		return time.Duration(*v)
	case int64:
		return time.Duration(v)
	case *int64:
		return time.Duration(*v)
	case uint:
		return time.Duration(v)
	case *uint:
		return time.Duration(*v)
	case uint8:
		return time.Duration(v)
	case *uint8:
		return time.Duration(*v)
	case uint16:
		return time.Duration(v)
	case *uint16:
		return time.Duration(*v)
	case uint32:
		return time.Duration(v)
	case *uint32:
		return time.Duration(*v)
	case uint64:
		return time.Duration(v)
	case *uint64:
		return time.Duration(*v)
	case float32:
		return time.Duration(v)
	case *float32:
		return time.Duration(*v)
	case float64:
		return time.Duration(v)
	case *float64:
		return time.Duration(*v)
	case complex64:
		return time.Duration(real(v))
	case *complex64:
		return time.Duration(real(*v))
	case complex128:
		return time.Duration(real(v))
	case *complex128:
		return time.Duration(real(*v))
	case bool:
		return 0
	case *bool:
		return 0
	case string:
		return toDuration(v)
	case *string:
		return toDuration(*v)
	case []byte:
		return toDuration(*(*string)(unsafe.Pointer(&v)))
	case *[]byte:
		return toDuration(*(*string)(unsafe.Pointer(v)))
	case time.Time:
		return time.Duration(v.UnixNano())
	case *time.Time:
		return time.Duration(v.UnixNano())
	case time.Duration:
		return v
	case *time.Duration:
		return *v
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
			return Duration(rv.Bool())
		case reflect.String:
			return Duration(rv.String())
		case reflect.Uintptr:
			return time.Duration(rv.Uint())
		case reflect.UnsafePointer:
			return time.Duration(rv.Pointer())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return time.Duration(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return time.Duration(rv.Uint())
		case reflect.Float32, reflect.Float64:
			return time.Duration(rv.Float())
		case reflect.Complex64, reflect.Complex128:
			return time.Duration(real(rv.Complex()))
		default:
			return 0
		}
	}
}

func Strings(any interface{}) (slice []string) {
	if any == nil {
		return
	}

	switch v := any.(type) {
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
	case []interface{}:
		slice = make([]string, len(v))
		for i := range v {
			slice[i] = String(v[i])
		}
	case *[]interface{}:
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
			slice = make([]string, count)
			for i := 0; i < count; i++ {
				slice[i] = String(rv.Index(i).Interface())
			}
		}
	}

	return
}
