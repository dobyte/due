// Package xconv 提供任意类型之间的转换工具，支持数值、字符串、布尔、
// 字节、符文、时长、存储容量等类型的互转及切片/指针形式转换
package xconv

import (
	"reflect"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// Anys 将任意值转换为 any 切片
// 仅支持切片或数组类型，其他类型返回 nil
// @param val any 待转换的值
// @return @1 []any 转换后的 any 切片
func Anys(val any) []any {
	if val == nil {
		return nil
	}

	switch rk, rv := xreflect.Value(val); rk {
	case reflect.Slice, reflect.Array:
		count := rv.Len()
		slice := make([]any, count)
		for i := range count {
			slice[i] = rv.Index(i).Interface()
		}
		return slice
	default:
		return nil
	}
}

// AnysPointer 将任意值转换为 any 切片指针
// @param val any 待转换的值
// @return @1 *[]any 转换后的 any 切片指针
func AnysPointer(val any) *[]any {
	v := Anys(val)
	return &v
}
