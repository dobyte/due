package xconv

import (
	"reflect"
	"strings"
	"time"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Bool 将任意值转换为布尔值
// 数值类型非零为 true；字符串非空、非"0"、非"false"（不区分大小写）为 true；
// 零值时间返回 false；切片/映射非 nil 且非空为 true；其他类型按反射结果判定
// @param val any 待转换的值
// @return @1 bool 转换后的布尔值
func Bool(val any) bool {
	if val == nil {
		return false
	}

	toBool := func(v string) bool {
		return v != "" && v != "0" && strings.ToLower(v) != "false"
	}

	switch v := val.(type) {
	case int:
		return v != 0
	case *int:
		return v != nil && *v != 0
	case int8:
		return v != 0
	case *int8:
		return v != nil && *v != 0
	case int16:
		return v != 0
	case *int16:
		return v != nil && *v != 0
	case int32:
		return v != 0
	case *int32:
		return v != nil && *v != 0
	case int64:
		return v != 0
	case *int64:
		return v != nil && *v != 0
	case uint:
		return v != 0
	case *uint:
		return v != nil && *v != 0
	case uint8:
		return v != 0
	case *uint8:
		return v != nil && *v != 0
	case uint16:
		return v != 0
	case *uint16:
		return v != nil && *v != 0
	case uint32:
		return v != 0
	case *uint32:
		return v != nil && *v != 0
	case uint64:
		return v != 0
	case *uint64:
		return v != nil && *v != 0
	case float32:
		return v != 0
	case *float32:
		return v != nil && *v != 0
	case float64:
		return v != 0
	case *float64:
		return v != nil && *v != 0
	case complex64:
		return toBool(String(v))
	case *complex64:
		return v != nil && toBool(String(*v))
	case complex128:
		return toBool(String(v))
	case *complex128:
		return v != nil && toBool(String(*v))
	case bool:
		return v
	case *bool:
		return v != nil && *v
	case string:
		return toBool(v)
	case *string:
		return v != nil && toBool(*v)
	case []byte:
		return toBool(BytesToString(v))
	case *[]byte:
		return v != nil && toBool(BytesToString(*v))
	case time.Time:
		return !v.IsZero()
	case *time.Time:
		return v != nil && !v.IsZero()
	default:
		switch rk, rv := xreflect.Value(val); rk {
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

// Bools 将任意值转换为布尔切片
// @param val any 待转换的值
// @return @1 []bool 转换后的布尔切片
func Bools(val any) (slice []bool) {
	if val == nil {
		return
	}

	switch v := val.(type) {
	case []int:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]int:
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	case []bool:
		return v
	case *[]bool:
		if v == nil {
			return
		}

		return *v
	case []any:
		slice = make([]bool, len(v))
		for i := range v {
			slice[i] = Bool(v[i])
		}
	case *[]any:
		if v == nil {
			return
		}

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
		if v == nil {
			return
		}

		slice = make([]bool, len(*v))
		for i := range *v {
			slice[i] = Bool((*v)[i])
		}
	default:
		switch rk, rv := xreflect.Value(val); rk {
		case reflect.Slice, reflect.Array:
			count := rv.Len()
			slice = make([]bool, count)
			for i := range count {
				slice[i] = Bool(rv.Index(i).Interface())
			}
		}
	}

	return
}

// BoolPointer 将任意值转换为布尔指针
// @param val any 待转换的值
// @return @1 *bool 转换后的布尔指针
func BoolPointer(any any) *bool {
	v := Bool(any)
	return &v
}

// BoolsPointer 将任意值转换为布尔切片指针
// @param val any 待转换的值
// @return @1 *[]bool 转换后的布尔切片指针
func BoolsPointer(any any) *[]bool {
	v := Bools(any)
	return &v
}
