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

// IsTelephone 检测是否是固定电话号码
// 支持带区号的格式（如 028-5554540、0285-55545401），也支持不带区号的 7~8 位号码
// @param telephone string 待检测的电话号码
// @return @1 bool 是否符合固定电话号码格式
func IsTelephone(telephone string) bool {
	matched, err := regexp.MatchString(`^((\d{3,4})|\d{3,4}-)?\d{7,8}$`, telephone)
	if err != nil {
		return false
	}

	return matched
}

// IsMobile 检测是否是手机号（国内）
// 校验 11 位手机号，覆盖 13、14、15、16、17、18、19 号段
// @param mobile string 待检测的手机号
// @return @1 bool 是否符合国内手机号格式
func IsMobile(mobile string) bool {
	matched, err := regexp.MatchString(`^13[\d]{9}$|^14[57]{1}\d{8}$|^15[^4]{1}\d{8}$|^16[2567]{1}\d{8}$|^17[0235678]{1}\d{8}$|^18[\d]{9}$|^19[\d]{9}$`, mobile)
	if err != nil {
		return false
	}

	return matched
}

// IsIdCard 检测是否是身份证号（国内18位）
// 校验地址码、出生年月日、顺序码及校验码
// 感谢知乎网友提供：https://zhuanlan.zhihu.com/p/608188853
// @param idCard string 待检测的身份证号
// @return @1 bool 是否是合法的身份证号
func IsIdCard(idCard string) bool {
	reg := regexp.MustCompile(`^[1-9]\d{5}\d{4}(0[1-9]|1[0-2])(0[1-9]|[1-2][0-9]|3[0-1])\d{3}([0-9Xx])$`)
	if !reg.MatchString(idCard) {
		return false
	}

	// 校验出生年份（1800年~今年）
	year, _ := strconv.Atoi(idCard[6:10])
	if year < 1800 || year > time.Now().Year() {
		return false
	}

	// 校验身份证号码校验码
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

// IsAccount 检测是否是账号
// 账号以字母开头，后续可包含字母、数字、下划线、连字符和点号，长度需在 min~max 之间
// @param account string 待检测的账号
// @param min int 账号最小长度
// @param max int 账号最大长度
// @return @1 bool 是否符合账号规则
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

// IsEmail 检测是否是邮箱
// @param email string 待检测的邮箱
// @return @1 bool 是否符合邮箱格式
func IsEmail(email string) bool {
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_\-\.]+@[a-zA-Z0-9_\-]+(\.[a-zA-Z0-9_\-]+)+$`, email)
	if err != nil {
		return false
	}

	return matched
}

// IsUrl 检测是否是URL
// 支持 http、https、ftp、file 协议，忽略大小写
// @param url string 待检测的URL
// @return @1 bool 是否符合URL格式
func IsUrl(url string) bool {
	matched, err := regexp.MatchString(`^(?i)(https?|ftp|file)://[-A-Za-z0-9+&@#/%?=~_|!:,.;]+[-A-Za-z0-9+&@#/%=~_|]$`, url)
	if err != nil {
		return false
	}

	return matched
}

// IsQQ 检测是否是QQ号
// QQ号为 5 位及以上数字，且首位不能为 0
// @param qq string 待检测的QQ号
// @return @1 bool 是否符合QQ号格式
func IsQQ(qq string) bool {
	matched, err := regexp.MatchString(`^[1-9][0-9]{4,}$`, qq)
	if err != nil {
		return false
	}

	return matched
}

// IsDigit 检测是否是数值
// 支持正负整数和正负浮点数，不包含前导零及科学计数法
// @param digit string 待检测的数值
// @return @1 bool 是否是合法的数值
func IsDigit(digit string) bool {
	matched, err := regexp.MatchString(`^-?(0|[1-9]\d*)(\.\d+)?$`, digit)
	if err != nil {
		return false
	}

	return matched
}

// IsNumber 检测是否是由数字组成的字符串
// 可选地指定长度：传入一个参数表示固定长度，传入两个参数表示最小、最大长度
// @param number string 待检测的字符串
// @param langths ...int 可选，长度参数
// @return @1 bool 是否是由数字组成的字符串
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

// In 检测值是否在给定的集合中
// 当 v 为切片或数组时，检测其中任一元素是否在集合中；否则检测 v 是否等于集合中的某个元素
// @param v any 待检测的值
// @param set any 集合（切片或数组）
// @return @1 bool 是否存在于集合中
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

// Between 检测字符串长度是否在设置的范围之间
// @param s string 待检测的字符串
// @param min int 最小长度
// @param max int 最大长度
// @return @1 bool 长度是否在 [min, max] 范围内
func Between(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return n >= min && n <= max
}

// Length 检测字符串长度是否等于固定长度
// @param s string 待检测的字符串
// @param n int 期望长度
// @return @1 bool 长度是否等于 n
func Length(s string, n int) bool {
	return utf8.RuneCountInString(s) == n
}

// MinLength 检测字符串的最小长度
// @param s string 待检测的字符串
// @param n int 最小长度
// @return @1 bool 长度是否不小于 n
func MinLength(s string, n int) bool {
	return utf8.RuneCountInString(s) >= n
}

// MaxLength 检测字符串的最大长度
// @param s string 待检测的字符串
// @param n int 最大长度
// @return @1 bool 长度是否不大于 n
func MaxLength(s string, n int) bool {
	return utf8.RuneCountInString(s) <= n
}
