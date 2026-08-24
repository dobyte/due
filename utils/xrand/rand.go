package xrand

import (
	"time"

	"github.com/dobyte/due/v2/core/rand"
)

// globalRand is the source of random numbers for the top-level
// convenience functions.
var globalRand = rand.NewRand(&rand.MathRand{})

const (
	LetterSeed           = rand.LetterSeed           // 大小写字母
	LetterLowerSeed      = rand.LetterLowerSeed      // 小写字母
	LetterUpperSeed      = rand.LetterUpperSeed      // 大写字母
	DigitSeed            = rand.DigitSeed            // 数字
	DigitWithoutZeroSeed = rand.DigitWithoutZeroSeed // 无0数字
	SymbolSeed           = rand.SymbolSeed           // 特殊字符
)

// Str 生成指定长度的字符串
// @param seed string 字符种子
// @param length int 字符串长度
// @return @1 string 生成的字符串
func Str(seed string, length int) string {
	return globalRand.Str(seed, length)
}

// Letters 生成指定长度的字母字符串
// @param length int 字符串长度
// @return @1 string 生成的字母字符串
func Letters(length int) string {
	return globalRand.Letters(length)
}

// Digits 生成指定长度的数字字符串
// @param length int 字符串长度
// @param hasLeadingZero ...bool 可选，是否允许以0开头
// @return @1 string 生成的数字字符串
func Digits(length int, hasLeadingZero ...bool) string {
	return globalRand.Digits(length, hasLeadingZero...)
}

// Symbols 生成指定长度的特殊字符字符串
// @param length int 字符串长度
// @return @1 string 生成的特殊字符字符串
func Symbols(length int) string {
	return globalRand.Symbols(length)
}

// IntR 生成[min,max]范围内的整数
// @param min int 最小值
// @param max int 最大值
// @return @1 int 生成的整数
func IntR(min, max int) int {
	return globalRand.IntR(min, max)
}

// Int32R 生成[min,max]范围内的32位整数
// @param min int32 最小值
// @param max int32 最大值
// @return @1 int32 生成的32位整数
func Int32R(min, max int32) int32 {
	return globalRand.Int32R(min, max)
}

// Int64R 生成[min,max]范围内的64位整数
// @param min int64 最小值
// @param max int64 最大值
// @return @1 int64 生成的64位整数
func Int64R(min, max int64) int64 {
	return globalRand.Int64R(min, max)
}

// Float32R 生成[min,max)范围内的32位浮点数
// @param min float32 最小值
// @param max float32 最大值
// @return @1 float32 生成的32位浮点数
func Float32R(min, max float32) float32 {
	return globalRand.Float32R(min, max)
}

// Float64R 生成[min,max)范围内的64位浮点数
// @param min float64 最小值
// @param max float64 最大值
// @return @1 float64 生成的64位浮点数
func Float64R(min, max float64) float64 {
	return globalRand.Float64R(min, max)
}

// Duration 生成[min,max]范围内的时间间隔
// @param min time.Duration 最小时间间隔
// @param max time.Duration 最大时间间隔
// @return @1 time.Duration 生成的时间间隔
func Duration(min, max time.Duration) time.Duration {
	return globalRand.Duration(min, max)
}

// Lucky 根据概率抽取幸运值
// @param probability float64 概率值
// @param base ...float64 可选概率基数，默认为100
// @return @1 bool 是否命中
func Lucky(probability float64, base ...float64) bool {
	return globalRand.Lucky(probability, base...)
}

// Weight 权重随机，权重合计无效（<=0）时返回 false
// @param fn func(v T) float64 权重计算函数
// @param list ...T 待抽取的元素列表
// @return @1 int 命中元素的下标，未命中时为-1
// @return @2 T 命中的元素，未命中时为零值
// @return @3 bool 是否命中
func Weight[T any](fn func(v T) float64, list ...T) (int, T, bool) {
	return globalRand.Weight(fn, list...)
}

// Shuffle 打乱切片
// @param list ...T 待打乱的切片
func Shuffle[T any](list ...T) {
	globalRand.Shuffle(list...)
}
