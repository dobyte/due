package xconv

import (
	"unsafe"
)

// StringToBytes 字符串无拷贝转字节数组
func StringToBytes(s string) (b []byte) {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// BytesToString 字节数组无拷贝转字符串
func BytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
