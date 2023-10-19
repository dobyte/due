package xrand

import (
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"
	"math"
	"math/rand"
	"strconv"
	"strings"
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

	return rand.Intn(max+1-min) + min
}

// Int32 生成[min,max]范围间的32位整数，
func Int32(min, max int32) int32 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	return rand.Int31n(max+1-min) + min
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

// Lucky 根据概率抽取幸运值
func Lucky(probability float64, base ...float64) bool {
	b := float64(100)
	if len(base) > 0 {
		b = base[0]
	}

	if probability >= b {
		return true
	}

	str := strconv.FormatFloat(probability, 'f', -1, 64)
	scale := float64(0)

	if i := strings.IndexByte(str, '.'); i > 0 {
		scale = math.Pow10(len(str) - i - 1)
	}

	return Int64(1, int64(b*scale)) <= int64(probability*scale)
}

// Weight 权重随机
func Weight(list []interface{}, fn func(v interface{}) float64) int {
	if len(list) == 0 {
		return -1
	}

	total := float64(0)
	scale := float64(0)

	for _, item := range list {
		weight := fn(item)
		str := strconv.FormatFloat(weight, 'f', -1, 64)

		if i := strings.IndexByte(str, '.'); i > 0 {
			scale = math.Max(scale, math.Pow10(len(str)-i-1))
		}

		total += weight
	}

	sum := int64(total * scale)

	if sum == 0 {
		return Int(1, len(list))
	}

	weight := Int64(1, sum)
	acc := int64(0)

	for i, item := range list {
		acc += int64(fn(item) * scale)
		if weight <= acc {
			return i
		}
	}

	return Int(1, len(list))
}
