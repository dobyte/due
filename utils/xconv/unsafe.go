package xconv

import (
	"unsafe"
)

// StringToBytes converts s to a byte slice without copying.
//
// The resulting byte slice shares its backing memory with the string; modifying the bytes would
// change the original string, so do not modify them.
func StringToBytes(s string) (b []byte) {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// BytesToString converts b to a string without copying.
//
// The resulting string shares its backing memory with the byte slice, so do not modify the
// original byte slice.
func BytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
