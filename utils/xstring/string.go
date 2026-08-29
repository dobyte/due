package xstring

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// firstRune 获取字符串的首个字符
// 若字符串为空或首个字节编码非法，返回 ok=false
// @param s string 待解析的字符串
// @return @1 rune 首个字符
// @return @2 bool 是否成功获取到有效字符
func firstRune(s string) (r rune, ok bool) {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 || (r == utf8.RuneError && size == 1) {
		return 0, false
	}
	return r, true
}

// FirstCharacterIsUpper 判断字符串的首字符是否是大写字母
// @param s string 待判断的字符串
// @return @1 bool 首字符是否为大写字母
func FirstCharacterIsUpper(s string) bool {
	r, ok := firstRune(s)
	return ok && unicode.IsUpper(r)
}

// FirstCharacterIsLower 判断字符串的首字符是否是小写字母
// @param s string 待判断的字符串
// @return @1 bool 首字符是否为小写字母
func FirstCharacterIsLower(s string) bool {
	r, ok := firstRune(s)
	return ok && unicode.IsLower(r)
}

// FirstCharacterIsNumber 判断字符串的首字符是否是数字
// @param s string 待判断的字符串
// @return @1 bool 首字符是否为数字
func FirstCharacterIsNumber(s string) bool {
	r, ok := firstRune(s)
	return ok && unicode.IsNumber(r)
}

// FirstCharacterIsSymbol 判断字符串的首字符是否是符号
// @param s string 待判断的字符串
// @return @1 bool 首字符是否为符号
func FirstCharacterIsSymbol(s string) bool {
	r, ok := firstRune(s)
	return ok && unicode.IsSymbol(r)
}

// Length 获取字符串长度
// 长度按字符数（rune）计算，而非字节数
// @param s string 待计算的字符串
// @return @1 int 字符串的字符数
func Length(s string) int {
	return utf8.RuneCountInString(s)
}

// PaddingPrefix 在字符串前面填充前缀，使结果长度达到 length
// 长度按字符数（rune）计算；当原字符串长度已不小于 length，或 padding 为空时，返回原字符串
// @param s string 原始字符串
// @param padding string 用于填充的字符串
// @param length int 目标长度
// @return @1 string 填充后的字符串
func PaddingPrefix(s, padding string, length int) string {
	paddingLen := length - utf8.RuneCountInString(s)

	if paddingLen <= 0 || padding == "" {
		return s
	}

	paddingRunes := []rune(padding)
	n := paddingLen / len(paddingRunes)
	remainder := paddingLen % len(paddingRunes)

	prefix := strings.Repeat(padding, n)
	if remainder > 0 {
		prefix += string(paddingRunes[:remainder])
	}

	return prefix + s
}

// PaddingSuffix 在字符串后面填充后缀，使结果长度达到 length
// 长度按字符数（rune）计算；当原字符串长度已不小于 length，或 padding 为空时，返回原字符串
// @param s string 原始字符串
// @param padding string 用于填充的字符串
// @param length int 目标长度
// @return @1 string 填充后的字符串
func PaddingSuffix(s, padding string, length int) string {
	paddingLen := length - utf8.RuneCountInString(s)

	if paddingLen <= 0 || padding == "" {
		return s
	}

	paddingRunes := []rune(padding)
	n := paddingLen / len(paddingRunes)
	remainder := paddingLen % len(paddingRunes)

	suffix := strings.Repeat(padding, n)
	if remainder > 0 {
		suffix += string(paddingRunes[:remainder])
	}

	return s + suffix
}

// Replace 替换字符串中指定范围的字符
// 从 start 位置（按字符下标，从 0 开始）开始，将连续 count 个字符替换为 replace 重复 count 次的结果；
// count 为负数时表示替换到字符串末尾；若 start 越界则返回原字符串
// @param str string 原始字符串
// @param start int 起始位置（字符下标）
// @param count int 替换的字符个数，负数表示替换到末尾
// @param replace string 用于替换的字符串
// @return @1 string 替换后的字符串
func Replace(str string, start, count int, replace string) string {
	s := []rune(str)

	if start < 0 || start >= len(s) {
		return str
	}

	if count < 0 {
		count = len(s) - start
	} else {
		if start+count >= len(s) {
			count = len(s) - start
		}
	}

	return string(s[:start]) + strings.Repeat(replace, count) + string(s[start+count:])
}
