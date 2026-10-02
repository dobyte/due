package xvalidate

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dobyte/due/v2/utils/xreflect"
)

// IsTelephone reports whether telephone is a fixed-line phone number. It accepts formats with an
// area code, such as 028-5554540 or 0285-55545401, as well as 7- to 8-digit numbers without one.
func IsTelephone(telephone string) bool {
	matched, err := regexp.MatchString(`^((\d{3,4})|\d{3,4}-)?\d{7,8}$`, telephone)
	if err != nil {
		return false
	}

	return matched
}

// IsMobile reports whether mobile is a Chinese mobile phone number. It validates an 11-digit number
// covering the 13, 14, 15, 16, 17, 18 and 19 prefixes.
func IsMobile(mobile string) bool {
	matched, err := regexp.MatchString(`^13[\d]{9}$|^14[57]{1}\d{8}$|^15[^4]{1}\d{8}$|^16[2567]{1}\d{8}$|^17[0235678]{1}\d{8}$|^18[\d]{9}$|^19[\d]{9}$`, mobile)
	if err != nil {
		return false
	}

	return matched
}

// IsIdCard reports whether idCard is a valid Chinese 18-digit resident identity card number. It
// validates the address code, the birth date, the sequence code and the check code.
//
// The check-code algorithm is taken from https://zhuanlan.zhihu.com/p/608188853.
func IsIdCard(idCard string) bool {
	reg := regexp.MustCompile(`^[1-9]\d{5}\d{4}(0[1-9]|1[0-2])(0[1-9]|[1-2][0-9]|3[0-1])\d{3}([0-9Xx])$`)
	if !reg.MatchString(idCard) {
		return false
	}

	// Validate the birth year, which must be between 1800 and the current year.
	year, _ := strconv.Atoi(idCard[6:10])
	if year < 1800 || year > time.Now().Year() {
		return false
	}

	// Validate the check code of the identity card number.
	factor := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checkCodes := []string{"1", "0", "X", "9", "8", "7", "6", "5", "4", "3", "2"}

	sum := 0
	for i := 0; i < 17; i++ {
		num, _ := strconv.Atoi(string(idCard[i]))
		sum += num * factor[i]
	}

	mod := sum % 11
	checkCode := checkCodes[mod]

	if strings.ToUpper(string(idCard[17])) != checkCode {
		return false
	}

	return true
}

// IsAccount reports whether account is a valid account name. An account starts with a letter, may
// then contain letters, digits, underscores, hyphens and dots, and its length must be between min
// and max inclusive.
func IsAccount(account string, min int, max int) bool {
	if min < 1 {
		min = 1
	}

	if max < min {
		return false
	}

	matched, err := regexp.MatchString(fmt.Sprintf(`^[a-zA-Z]{1}[a-zA-Z0-9_\-\.]{%d,%d}$`, min-1, max-1), account)
	if err != nil {
		return false
	}

	return matched
}

// IsEmail reports whether email is a valid email address.
func IsEmail(email string) bool {
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_\-\.]+@[a-zA-Z0-9_\-]+(\.[a-zA-Z0-9_\-]+)+$`, email)
	if err != nil {
		return false
	}

	return matched
}

// IsUrl reports whether url is a valid URL. The http, https, ftp and file schemes are supported and
// matched case-insensitively.
func IsUrl(url string) bool {
	matched, err := regexp.MatchString(`^(?i)(https?|ftp|file)://[-A-Za-z0-9+&@#/%?=~_|!:,.;]+[-A-Za-z0-9+&@#/%=~_|]$`, url)
	if err != nil {
		return false
	}

	return matched
}

// IsQQ reports whether qq is a valid QQ number. A QQ number has at least 5 digits and its first
// digit is not zero.
func IsQQ(qq string) bool {
	matched, err := regexp.MatchString(`^[1-9][0-9]{4,}$`, qq)
	if err != nil {
		return false
	}

	return matched
}

// IsDigit reports whether digit is a valid numeric value. Positive and negative integers and floats
// are supported, but leading zeros and scientific notation are not.
func IsDigit(digit string) bool {
	matched, err := regexp.MatchString(`^-?(0|[1-9]\d*)(\.\d+)?$`, digit)
	if err != nil {
		return false
	}

	return matched
}

// IsNumber reports whether number consists only of digits. Its length may optionally be
// constrained: a single langths argument requires an exact length, while two arguments require a
// minimum and a maximum length.
func IsNumber(number string, langths ...int) bool {
	var pattern string
	switch len(langths) {
	case 0:
		pattern = `^\d+$`
	case 1:
		pattern = fmt.Sprintf(`^\d{%d}$`, langths[0])
	default:
		if langths[0] > langths[1] {
			return false
		}
		pattern = fmt.Sprintf(`^\d{%d,%d}$`, langths[0], langths[1])
	}

	matched, err := regexp.MatchString(pattern, number)
	if err != nil {
		return false
	}

	return matched
}

// In reports whether v is contained in set. When v is a slice or an array, it reports whether any
// of its elements is in the set; otherwise it reports whether v equals an element of the set.
func In(v any, set any) bool {
	kind, value := xreflect.Value(set)
	if kind != reflect.Slice && kind != reflect.Array {
		return false
	}

	if value.Len() == 0 {
		return false
	}

	kk, vv := xreflect.Value(v)

	if kk == reflect.Slice || kk == reflect.Array {
		check := make(map[any]struct{}, value.Len())

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

// Between reports whether the length of s is within [min, max].
func Between(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return n >= min && n <= max
}

// Length reports whether the length of s equals n.
func Length(s string, n int) bool {
	return utf8.RuneCountInString(s) == n
}

// MinLength reports whether the length of s is at least n.
func MinLength(s string, n int) bool {
	return utf8.RuneCountInString(s) >= n
}

// MaxLength reports whether the length of s is at most n.
func MaxLength(s string, n int) bool {
	return utf8.RuneCountInString(s) <= n
}
