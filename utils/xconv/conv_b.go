package xconv

import (
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// byteRegexp matches a storage-capacity string such as "10KB", "5M" or "1.5GB", where the numeric
// part may be a decimal.
var byteRegexp = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)(b|k|m|g|t|p|e|z|kb|mb|gb|tb|pb|eb|zb)?$`)

// toB parses a storage-capacity string as a number of bytes.
//
// It accepts the units b/k/m/g/t/p/e/z, with or without the trailing b (case-insensitive), and
// the numeric part may be a decimal such as 1.5GB. It returns 0 when the string cannot be parsed.
func toB(val string) float64 {
	if rst := byteRegexp.FindStringSubmatch(val); len(rst) == 3 {
		var unit float64

		switch strings.ToUpper(rst[2]) {
		case "", "B":
			unit = 1
		case "K", "KB":
			unit = 1 << 10
		case "M", "MB":
			unit = 1 << 20
		case "G", "GB":
			unit = 1 << 30
		case "T", "TB":
			unit = 1 << 40
		case "P", "PB":
			unit = 1 << 50
		case "E", "EB":
			unit = 1 << 60
		case "Z", "ZB":
			unit = 1 << 70
		default:
			return 0
		}

		return Float64(rst[1]) * unit
	} else {
		return 0
	}
}

// B converts val to a number of storage-capacity bytes as a float64.
//
// Numeric values are returned as is; strings accept capacity units such as "10KB" or "5M"
// (case-insensitive); the conversion returns 0 when it fails.
func B(val any) float64 {
	switch v := val.(type) {
	case int:
		return float64(v)
	case *int:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case int8:
		return float64(v)
	case *int8:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case int16:
		return float64(v)
	case *int16:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case int32:
		return float64(v)
	case *int32:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case int64:
		return float64(v)
	case *int64:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case uint:
		return float64(v)
	case *uint:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case uint8:
		return float64(v)
	case *uint8:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case uint16:
		return float64(v)
	case *uint16:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case uint32:
		return float64(v)
	case *uint32:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case uint64:
		return float64(v)
	case *uint64:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case float32:
		return float64(v)
	case *float32:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case float64:
		return v
	case *float64:
		if v == nil {
			return 0
		} else {
			return float64(*v)
		}
	case complex64:
		return float64(real(v))
	case *complex64:
		if v == nil {
			return 0
		} else {
			return float64(real(*v))
		}
	case complex128:
		return float64(real(v))
	case *complex128:
		if v == nil {
			return 0
		} else {
			return float64(real(*v))
		}
	case bool:
		return 0
	case *bool:
		return 0
	case string:
		return toB(v)
	case *string:
		if v == nil {
			return 0
		} else {
			return toB(*v)
		}
	case []byte:
		return toB(BytesToString(v))
	case *[]byte:
		if v == nil {
			return 0
		} else {
			return toB(BytesToString(*v))
		}
	case time.Time:
		return 0
	case *time.Time:
		return 0
	case time.Duration:
		return 0
	case *time.Duration:
		return 0
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.String:
			return toB(rv.String())
		case reflect.Uintptr:
			return float64(rv.Uint())
		case reflect.UnsafePointer:
			return float64(rv.Pointer())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return float64(rv.Uint())
		case reflect.Float32, reflect.Float64:
			return float64(rv.Float())
		case reflect.Complex64, reflect.Complex128:
			return float64(real(rv.Complex()))
		default:
			return 0
		}
	}
}

// Bs converts val to a slice of storage-capacity bytes as float64.
func Bs(val any) (slice []float64) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]int:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []int8:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]int8:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []int16:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]int16:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []int32:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]int32:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []int64:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]int64:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []uint:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]uint:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []uint8:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]uint8:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []uint16:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]uint16:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []uint32:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]uint32:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []uint64:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]uint64:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []float32:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]float32:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []float64:
		return v
	case *[]float64:
		if v == nil {
			return
		}

		return *v
	case []complex64:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]complex64:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []complex128:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]complex128:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []string:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]string:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case []bool:
		return make([]float64, len(v))
	case *[]bool:
		if v == nil {
			return
		}
		return make([]float64, len(*v))
	case []any:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[]any:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	case [][]byte:
		slice = make([]float64, len(v))
		for i := range v {
			slice[i] = B(v[i])
		}
	case *[][]byte:
		if v == nil {
			return
		}

		slice = make([]float64, len(*v))
		for i := range *v {
			slice[i] = B((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]float64, count)
			for i := range count {
				slice[i] = B(rv.Index(i).Interface())
			}
		}
	}

	return
}
