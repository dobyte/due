package xstring

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// firstRune returns the first character of s. It reports ok as false when s is empty or its first
// byte is an invalid UTF-8 encoding.
func firstRune(s string) (r rune, ok bool) {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 || (r == utf8.RuneError && size == 1) {
		return 0, false
	}
	return r, true
}

// FirstCharacterIsUpper reports whether the first character of s is an upper-case letter.
func FirstCharacterIsUpper(s string) bool {
	r, ok := firstRune(s)
	return ok && unicode.IsUpper(r)
}

// FirstCharacterIsLower reports whether the first character of s is a lower-case letter.
func FirstCharacterIsLower(s string) bool {
	r, ok := firstRune(s)
	return ok && unicode.IsLower(r)
}

// FirstCharacterIsNumber reports whether the first character of s is a digit.
func FirstCharacterIsNumber(s string) bool {
	r, ok := firstRune(s)
	return ok && unicode.IsNumber(r)
}

// FirstCharacterIsSymbol reports whether the first character of s is a symbol.
func FirstCharacterIsSymbol(s string) bool {
	r, ok := firstRune(s)
	return ok && unicode.IsSymbol(r)
}

// Length returns the number of characters (runes) in s rather than its number of bytes.
func Length(s string) int {
	return utf8.RuneCountInString(s)
}

// PaddingPrefix prepends padding to s until the result reaches length characters. Lengths are
// counted in runes. It returns s unchanged when s already has at least length characters or when
// padding is empty.
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

// PaddingSuffix appends padding to s until the result reaches length characters. Lengths are
// counted in runes. It returns s unchanged when s already has at least length characters or when
// padding is empty.
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

// Replace replaces count characters of str with replace repeated count times, starting at the
// character index start (0-based). A negative count means replacing through the end of the string.
// It returns str unchanged when start is out of range.
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
