package xrand

import (
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/utils/xtime"
	"math/rand"
)

const (
	LetterSeed           = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" // 字母字
	LetterLowerSeed      = "abcdefghijklmnopqrstuvwxyz"                           // 小写字母
	LetterUpperSeed      = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"                           // 大写字母
	DigitSeed            = "0123456789"                                           // 数字
	DigitWithoutZeroSeed = "123456789"                                            // 无0数字
	SymbolSeed           = "!\\\"#$%&'()*+,-./:;<=>?@[\\\\]^_`{|}~"               // 特殊字符
)

func init() {
	rand.Seed(xtime.Now().UnixNano())
}

// Str 生成指定长度的字符串
func Str(seed string, length int) (str string) {
	r := []rune(seed)
	n := len(r)
	if n == 0 {
		log.Warnf("invalid seed")
		return
	}

	for i := 0; i < length; i++ {
		pos := rand.Intn(n)
		str += string(r[pos : pos+1])
	}

	return
}

// Letters 生成指定长度的字母字符串
func Letters(length int) string {
	return Str(LetterSeed, length)
}

// Digits 生成指定长度的数字字符串
func Digits(length int) string {
	return Str(DigitSeed, length)
}

// Symbols 生成指定长度的特殊字符串
func Symbols(length int) string {
	return Str(SymbolSeed, length)
}

// Int 生成[min,max]的整数
// min -50 max 100
func Int(min, max int) int {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	return rand.Intn(max-min) + min
}

// Int32 生成[min,max]范围间的32位整数，
func Int32(min, max int32) int32 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	return rand.Int31n(max-min) + min
}

// Int64 生成[min,max]范围间的64位整数
func Int64(min, max int64) int64 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	return rand.Int63n(max+1-min) + min
}

// Float32 生成[min,max)范围间的32位浮点数
func Float32(min, max float32) float32 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	return min + rand.Float32()*(max-min)
}

// Float64 生成[min,max)范围间的64位浮点数
func Float64(min, max float64) float64 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	return min + rand.Float64()*(max-min)
}
