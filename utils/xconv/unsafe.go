package xconv

import (
	"unsafe"
)

// StringToBytes 字符串无拷贝转字节切片
// 转换后的字节切片与字符串共享底层内存，修改字节会改变原字符串，请勿修改
// @param s string 待转换的字符串
// @return @1 []byte 转换后的字节切片
func StringToBytes(s string) (b []byte) {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// BytesToString 字节切片无拷贝转字符串
// 转换后的字符串与字节切片共享底层内存，请勿修改原字节切片
// @param b []byte 待转换的字节切片
// @return @1 string 转换后的字符串
func BytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
