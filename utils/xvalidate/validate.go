package xvalidate

import (
	"fmt"
	"github.com/dobyte/due/v2/utils/xreflect"
	"reflect"
	"regexp"
	"unicode/utf8"
)

// IsTelephone 检测是否是电话号码
func IsTelephone(telephone string) bool {
	matched, err := regexp.MatchString(`^((\d{3,4})|\d{3,4}-)?\d{7,8}$`, telephone)
	if err != nil {
		return false
	}

	return matched
}

// IsMobile 检测是否是手机号（国内）
func IsMobile(mobile string) bool {
	matched, err := regexp.MatchString(`^13[\d]{9}$|^14[5,7]{1}\d{8}$|^15[^4]{1}\d{8}$|^16[\d]{9}$|^17[0,2,3,5,6,7,8]{1}\d{8}$|^18[\d]{9}$|^19[\d]{9}$`, mobile)
	if err != nil {
		return false
	}

	return matched
}

// IsAccount 检测是否是账号
func IsAccount(account string, min int, max int) bool {
	matched, err := regexp.MatchString(fmt.Sprintf(`^[a-zA-Z]{1}[a-zA-Z0-9_\-\.]{%d,%d}$`, min-1, max-1), account)
	if err != nil {
		return false
	}

	return matched
}

// IsEmail 检测是否是邮箱
func IsEmail(email string) bool {
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_\-\.]+@[a-zA-Z0-9_\-]+(\.[a-zA-Z0-9_\-]+)+$`, email)
	if err != nil {
		return false
	}

	return matched
}

// IsUrl 检测是否是URL
func IsUrl(url string) bool {
	matched, err := regexp.MatchString(`(?i)(https?|ftp|file)://[-A-Za-z0-9+&@#/%?=~_|!:,.;]+[-A-Za-z0-9+&@#/%=~_|]`, url)
	if err != nil {
		return false
	}

	return matched
}

// IsQQ 是否是QQ号
func IsQQ(qq string) bool {
	matched, err := regexp.MatchString(`^[1-9][0-9]{4,}$`, qq)
	if err != nil {
		return false
	}

	return matched
}

// IsDigit 检测是否是数值（正负整数、正负浮点数）
func IsDigit(digit string) bool {
	matched, err := regexp.MatchString(`^-?[1-9]\d*(.\d)*$`, digit)
	if err != nil {
		return false
	}

	return matched
}

// IsNumber 检测是否是数字
func IsNumber(number string, langths ...int) bool {
	var pattern string
	switch len(langths) {
	case 0:
		pattern = `^\d+$`
	case 1:
		pattern = fmt.Sprintf(`^\d{%d}$`, langths[0])
	default:
		pattern = fmt.Sprintf(`^\d{%d,%d}$`, langths[0], langths[1])
	}

	matched, err := regexp.MatchString(pattern, number)
	if err != nil {
		return false
	}

	return matched
}

// In 检测是值是否在给定的集合中
func In(v interface{}, set interface{}) bool {
	kind, value := xreflect.Value(set)
	if kind != reflect.Slice && kind != reflect.Array {
		return false
	}

	if value.Len() == 0 {
		return false
	}

	kk, vv := xreflect.Value(v)

	if kk == reflect.Slice || kk == reflect.Array {
		check := make(map[interface{}]struct{}, value.Len())

		for i := 0; i < value.Len(); i++ {
			val := value.Index(i)

			if !val.Comparable() {
				continue
			}

			check[val.Interface()] = struct{}{}
		}

		for i := 0; i < vv.Len(); i++ {
			val := vv.Index(i)

			if !val.Comparable() {
				continue
			}

			if _, ok := check[val.Interface()]; ok {
				return true
			}
		}
	} else {
		if !vv.Comparable() {
			return false
		}

		for i := 0; i < value.Len(); i++ {
			val := value.Index(i)

			if !val.Comparable() {
				continue
			}

			if reflect.DeepEqual(vv.Interface(), val.Interface()) {
				return true
			}
		}
	}

	return false
}

// Between 检测字符串长度是否在设置的范围之间
func Between(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return n >= min && n <= max
}

// Length 检测字符串长度是否等于固定长度
func Length(s string, n int) bool {
	return utf8.RuneCountInString(s) == n
}

// MinLength 检测字符串的最小长度
func MinLength(s string, n int) bool {
	return utf8.RuneCountInString(s) >= n
}

// MaxLength 检测字符串的最大长度
func MaxLength(s string, n int) bool {
	return utf8.RuneCountInString(s) <= n
}
