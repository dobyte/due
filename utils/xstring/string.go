package xstring

import (
	"unicode"
	"unicode/utf8"
)

// FirstCharacterIsUpper 首字符是否是大写
func FirstCharacterIsUpper(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return r != utf8.RuneError && unicode.IsUpper(r)
}

// FirstCharacterIsLower 首字符是否是小写
func FirstCharacterIsLower(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return r != utf8.RuneError && unicode.IsLower(r)
}

// FirstCharacterIsNumber 首字符是否是数字
func FirstCharacterIsNumber(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return r != utf8.RuneError && unicode.IsNumber(r)
}

// FirstCharacterIsSymbol 首字符是否是符号
func FirstCharacterIsSymbol(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return r != utf8.RuneError && unicode.IsSymbol(r)
}

// Length 获取字符串长度
func Length(s string) int {
	return utf8.RuneCountInString(s)
}
