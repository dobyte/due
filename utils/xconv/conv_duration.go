package xconv

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dobyte/due/v2/utils/xreflect"
)

var durationRegexp = regexp.MustCompile(`(((-?\d+)(\.\d+)?)(d))`)

// Duration converts val to a time.Duration.
//
// Numeric values are converted directly as nanoseconds; strings accept time units such as "ns",
// "us" (or "µs"), "ms", "s", "m", "h" and "d" (day), where "d" is replaced by the corresponding
// number of nanoseconds before parsing.
func Duration(val any) time.Duration {
	if val == nil {
		return 0
	}

	// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h", "d".
	toDuration := func(s string) time.Duration {
		d, _ := time.ParseDuration(durationRegexp.ReplaceAllStringFunc(strings.ToLower(s), func(ss string) string {
			v, err := strconv.ParseFloat(strings.TrimSuffix(ss, "d"), 64)
			if err != nil {
				return ""
			}
			return fmt.Sprintf("%dns", int64(v*24*3600*1000*1000*1000))
		}))
		return d
	}

	switch v := val.(type) {
	case int:
		return time.Duration(v)
	case *int:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case int8:
		return time.Duration(v)
	case *int8:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case int16:
		return time.Duration(v)
	case *int16:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case int32:
		return time.Duration(v)
	case *int32:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case int64:
		return time.Duration(v)
	case *int64:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case uint:
		return time.Duration(v)
	case *uint:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case uint8:
		return time.Duration(v)
	case *uint8:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case uint16:
		return time.Duration(v)
	case *uint16:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case uint32:
		return time.Duration(v)
	case *uint32:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case uint64:
		return time.Duration(v)
	case *uint64:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case float32:
		return time.Duration(v)
	case *float32:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case float64:
		return time.Duration(v)
	case *float64:
		if v == nil {
			return 0
		}
		return time.Duration(*v)
	case complex64:
		return time.Duration(real(v))
	case *complex64:
		if v == nil {
			return 0
		}
		return time.Duration(real(*v))
	case complex128:
		return time.Duration(real(v))
	case *complex128:
		if v == nil {
			return 0
		}
		return time.Duration(real(*v))
	case bool:
		return 0
	case *bool:
		if v == nil {
			return 0
		}
		return 0
	case string:
		return toDuration(v)
	case *string:
		if v == nil {
			return 0
		}
		return toDuration(*v)
	case []byte:
		return toDuration(BytesToString(v))
	case *[]byte:
		if v == nil {
			return 0
		}
		return toDuration(BytesToString(*v))
	case time.Time:
		return time.Duration(v.UnixNano())
	case *time.Time:
		if v == nil {
			return 0
		}
		return time.Duration(v.UnixNano())
	case time.Duration:
		return v
	case *time.Duration:
		if v == nil {
			return 0
		}
		return *v
	default:
		switch rk, rv := xreflect.Value(val); rk {
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

// Durations converts val to a time.Duration slice.
func Durations(val any) (slice []time.Duration) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]int:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []int8:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]int8:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []int16:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]int16:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []int32:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]int32:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []int64:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]int64:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []uint:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]uint:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []uint8:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]uint8:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []uint16:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]uint16:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []uint32:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]uint32:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []uint64:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]uint64:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []float32:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]float32:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []float64:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]float64:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []complex64:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]complex64:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []complex128:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]complex128:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []string:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]string:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []bool:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]bool:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case []any:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[]any:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	case [][]byte:
		slice = make([]time.Duration, len(v))
		for i := range v {
			slice[i] = Duration(v[i])
		}
	case *[][]byte:
		if v == nil {
			return
		}
		slice = make([]time.Duration, len(*v))
		for i := range *v {
			slice[i] = Duration((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]time.Duration, count)
			for i := range count {
				slice[i] = Duration(rv.Index(i).Interface())
			}
		}
	}

	return
}

// DurationPointer converts val to a pointer to time.Duration.
func DurationPointer(any any) *time.Duration {
	v := Duration(any)
	return &v
}

// DurationsPointer converts val to a pointer to a time.Duration slice.
func DurationsPointer(any any) *[]time.Duration {
	v := Durations(any)
	return &v
}
