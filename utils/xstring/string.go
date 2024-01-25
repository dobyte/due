package xstring

import (
	"math"
	"strings"
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

// PaddingPrefix 填充前缀
func PaddingPrefix(s, padding string, length int) string {
	paddingLen := length - len(s)

	if paddingLen <= 0 {
		return s
	}

	n := int(math.Ceil(float64(paddingLen) / float64(len(padding))))

	return strings.Repeat(padding, n)[:paddingLen] + s
}

// PaddingSuffix 填充后缀
func PaddingSuffix(s, padding string, length int) string {
	paddingLen := length - len(s)

	if paddingLen <= 0 {
		return s
	}

	n := int(math.Ceil(float64(paddingLen) / float64(len(padding))))

	return s + strings.Repeat(padding, n)[:paddingLen]
}
