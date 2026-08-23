package xrand

import (
	"math/rand"
	"strings"
	"time"

	"github.com/dobyte/due/v2/log"
)

const (
	LetterSeed           = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" // 字母字
	LetterLowerSeed      = "abcdefghijklmnopqrstuvwxyz"                           // 小写字母
	LetterUpperSeed      = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"                           // 大写字母
	DigitSeed            = "0123456789"                                           // 数字
	DigitWithoutZeroSeed = "123456789"                                            // 无0数字
	SymbolSeed           = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"                   // 特殊字符
)

// Str 生成指定长度的字符串
func Str(seed string, length int) string {
	if length <= 0 {
		return ""
	}

	r := []rune(seed)
	n := len(r)
	if n == 0 {
		log.Warnf("invalid seed")
		return ""
	}

	builder := strings.Builder{}
	builder.Grow(length)

	for range length {
		builder.WriteRune(r[rand.Intn(n)])
	}

	return builder.String()
}

// Letters 生成指定长度的字母字符串
func Letters(length int) string {
	return Str(LetterSeed, length)
}

// Digits 生成指定长度的数字字符串
func Digits(length int, hasLeadingZero ...bool) string {
	if length <= 0 {
		return ""
	}

	if len(hasLeadingZero) > 0 && hasLeadingZero[0] {
		return Str(DigitSeed, length)
	}

	if length == 1 {
		return Str(DigitWithoutZeroSeed, 1)
	}

	return Str(DigitWithoutZeroSeed, 1) + Str(DigitSeed, length-1)
}

// Symbols 生成指定长度的特殊字符串
func Symbols(length int) string {
	return Str(SymbolSeed, length)
}

// Int 生成[min,max]的整数
func Int(min, max int) int {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	span := uint64(max) - uint64(min) + 1
	if span == 0 {
		return int(rand.Uint64())
	}

	// 拒绝采样，避免 max+1 溢出导致的 panic 及取模偏差
	limit := ^uint64(0) - (^uint64(0) % span)

	for {
		r := rand.Uint64()
		if r >= limit {
			continue
		}

		return int(uint64(min) + r%span)
	}
}

// Int32 生成[min,max]范围间的32位整数，
func Int32(min, max int32) int32 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	span := uint32(max) - uint32(min) + 1
	if span == 0 {
		return int32(rand.Uint32())
	}

	limit := ^uint32(0) - (^uint32(0) % span)

	for {
		r := rand.Uint32()
		if r >= limit {
			continue
		}

		return int32(uint32(min) + r%span)
	}
}

// Int64 生成[min,max]范围间的64位整数
func Int64(min, max int64) int64 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	span := uint64(max) - uint64(min) + 1
	if span == 0 {
		return int64(rand.Uint64())
	}

	limit := ^uint64(0) - (^uint64(0) % span)

	for {
		r := rand.Uint64()
		if r >= limit {
			continue
		}

		return int64(uint64(min) + r%span)
	}
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

// Duration 生成[min,max]范围间的时间间隔
func Duration(min, max time.Duration) time.Duration {
	return time.Duration(Int64(int64(min), int64(max)))
}

// Lucky 根据概率抽取幸运值
func Lucky(probability float64, base ...float64) bool {
	if probability <= 0 {
		return false
	}

	b := float64(100)

	if len(base) > 0 {
		if base[0] <= 0 {
			return false
		} else {
			b = base[0]
		}
	}

	if probability >= b {
		return true
	}

	return rand.Float64() < probability/b
}

// Weight 权重随机；权重合计无效（<=0）时返回 false
func Weight[T any](fn func(v T) float64, list ...T) (int, T, bool) {
	var v T

	if len(list) == 0 {
		return -1, v, false
	}

	var (
		total   = float64(0)
		weights = make([]float64, len(list))
	)

	for i, item := range list {
		weight := fn(item)
		weights[i] = weight

		if weight > 0 {
			total += weight
		}
	}

	if total <= 0 {
		return -1, v, false
	}

	r := rand.Float64() * total
	acc := float64(0)

	for i, w := range weights {
		if w <= 0 {
			continue
		}

		acc += w
		if r < acc {
			return i, list[i], true
		}
	}

	return -1, v, false
}

// Shuffle 打乱数组
func Shuffle[T any](list []T) {
	rand.Shuffle(len(list), func(i, j int) {
		list[i], list[j] = list[j], list[i]
	})
}
