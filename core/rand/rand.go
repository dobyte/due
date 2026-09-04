package rand

import (
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/log"
)

const (
	LetterSeed           = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" // 大小写字母
	LetterLowerSeed      = "abcdefghijklmnopqrstuvwxyz"                           // 小写字母
	LetterUpperSeed      = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"                           // 大写字母
	DigitSeed            = "0123456789"                                           // 数字
	DigitWithoutZeroSeed = "123456789"                                            // 无0数字
	SymbolSeed           = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"                   // 特殊字符
)

type intType interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

type Rand struct {
	mu  *sync.Mutex
	rng Random
}

// NewRand 创建一个新的随机数生成器
// @param rng Random 随机数生成器实例（如*rand.Rand、*PseudoRandom或*TrueRandom等实现了Random接口的类型）
// @param safe ...bool 可选，是否启用并发安全模式（使用互斥锁）
// @return @1 *Rand 随机数生成器实例
func NewRand(rng Random, safe ...bool) *Rand {
	rd := &Rand{}
	rd.rng = rng
	if len(safe) > 0 && safe[0] {
		rd.mu = &sync.Mutex{}
	}

	return rd
}

// Int64 返回一个非负的伪随机63位整数
// @return @1 int64 生成的随机整数
func (r *Rand) Int64() int64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Int64()
}

// Uint32 返回一个伪随机32位无符号整数
// @return @1 uint32 生成的随机无符号整数
func (r *Rand) Uint32() uint32 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Uint32()
}

// Uint64 返回一个伪随机64位无符号整数
// @return @1 uint64 生成的随机无符号整数
func (r *Rand) Uint64() uint64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Uint64()
}

// Int32 返回一个非负的伪随机31位整数
// @return @1 int32 生成的随机整数
func (r *Rand) Int32() int32 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Int32()
}

// Int 返回一个非负的伪随机整数
// @return @1 int 生成的随机整数
func (r *Rand) Int() int {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Int()
}

// Uint 返回一个伪随机无符号整数
// @return @1 uint 生成的随机无符号整数
func (r *Rand) Uint() uint {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Uint()
}

// Int64N 返回[0,n)范围内的非负伪随机64位整数
// @param n int64 范围上限（不包含），n必须大于0
// @return @1 int64 生成的随机整数
func (r *Rand) Int64N(n int64) int64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Int64N(n)
}

// Uint64N 返回[0,n)范围内的非负伪随机64位无符号整数
// @param n uint64 范围上限（不包含），n必须大于0
// @return @1 uint64 生成的随机无符号整数
func (r *Rand) Uint64N(n uint64) uint64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Uint64N(n)
}

// Int32N 返回[0,n)范围内的非负伪随机32位整数
// @param n int32 范围上限（不包含），n必须大于0
// @return @1 int32 生成的随机整数
func (r *Rand) Int32N(n int32) int32 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Int32N(n)
}

// Uint32N 返回[0,n)范围内的非负伪随机32位无符号整数
// @param n uint32 范围上限（不包含），n必须大于0
// @return @1 uint32 生成的随机无符号整数
func (r *Rand) Uint32N(n uint32) uint32 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Uint32N(n)
}

// IntN 返回[0,n)范围内的非负伪随机整数
// @param n int 范围上限（不包含），n必须大于0
// @return @1 int 生成的随机整数
func (r *Rand) IntN(n int) int {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.IntN(n)
}

// UintN 返回[0,n)范围内的非负伪随机无符号整数
// @param n uint 范围上限（不包含），n必须大于0
// @return @1 uint 生成的随机无符号整数
func (r *Rand) UintN(n uint) uint {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.UintN(n)
}

// Float64 返回[0.0,1.0)范围内的伪随机64位浮点数
// @return @1 float64 生成的随机浮点数
func (r *Rand) Float64() float64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Float64()
}

// Float32 返回[0.0,1.0)范围内的伪随机32位浮点数
// @return @1 float32 生成的随机浮点数
func (r *Rand) Float32() float32 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Float32()
}

// Perm 返回[0,n)范围内整数的伪随机排列切片
// @param n int 排列长度
// @return @1 []int 随机排列后的整数切片
func (r *Rand) Perm(n int) []int {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Perm(n)
}

// ExpFloat64 返回一个服从指数分布的64位浮点数，速率参数(lambda)为1，均值为1
// @return @1 float64 生成的指数分布随机数
func (r *Rand) ExpFloat64() float64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.ExpFloat64()
}

// NormFloat64 返回一个服从标准正态分布的64位浮点数（均值=0，标准差=1）
// @return @1 float64 生成的正态分布随机数
func (r *Rand) NormFloat64() float64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.NormFloat64()
}

// Str 生成指定长度的字符串
// @param seed string 字符种子
// @param length int 字符串长度
// @return @1 string 生成的字符串
func (r *Rand) Str(seed string, length int) string {
	if length <= 0 {
		return ""
	}

	s := []rune(seed)
	n := len(s)
	if n == 0 {
		log.Warnf("invalid seed")
		return ""
	}

	builder := strings.Builder{}
	builder.Grow(length)

	if r.mu != nil {
		r.mu.Lock()
		for range length {
			builder.WriteRune(s[r.rng.IntN(n)])
		}
		r.mu.Unlock()
	} else {
		for range length {
			builder.WriteRune(s[r.rng.IntN(n)])
		}
	}

	return builder.String()
}

// Letters 生成指定长度的字母字符串
// @param length int 字符串长度
// @return @1 string 生成的字母字符串
func (r *Rand) Letters(length int) string {
	return r.Str(LetterSeed, length)
}

// Digits 生成指定长度的数字字符串
// @param length int 字符串长度
// @param hasLeadingZero ...bool 可选，是否允许以0开头
// @return @1 string 生成的数字字符串
func (r *Rand) Digits(length int, hasLeadingZero ...bool) string {
	if length <= 0 {
		return ""
	}

	if len(hasLeadingZero) > 0 && hasLeadingZero[0] {
		return r.Str(DigitSeed, length)
	}

	if length == 1 {
		return r.Str(DigitWithoutZeroSeed, 1)
	}

	return r.Str(DigitWithoutZeroSeed, 1) + r.Str(DigitSeed, length-1)
}

// Symbols 生成指定长度的特殊字符字符串
// @param length int 字符串长度
// @return @1 string 生成的特殊字符字符串
func (r *Rand) Symbols(length int) string {
	return r.Str(SymbolSeed, length)
}

// IntR 生成[min,max]范围内的整数
// @param min int 最小值
// @param max int 最大值
// @return @1 int 生成的整数
func (rd *Rand) IntR(min, max int) int {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	span := uint64(max) - uint64(min) + 1
	if span == 0 {
		return int(rd.Uint64())
	}

	// 拒绝采样，避免 max+1 溢出导致的 panic 及取模偏差
	limit := ^uint64(0) - (^uint64(0) % span)

	for {
		r := rd.Uint64()
		if r >= limit {
			continue
		}

		return int(uint64(min) + r%span)
	}
}

// Int32R 生成[min,max]范围内的32位整数
// @param min int32 最小值
// @param max int32 最大值
// @return @1 int32 生成的32位整数
func (rd *Rand) Int32R(min, max int32) int32 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	span := uint32(max) - uint32(min) + 1
	if span == 0 {
		return int32(rd.Uint32())
	}

	limit := ^uint32(0) - (^uint32(0) % span)

	for {
		r := rd.Uint32()
		if r >= limit {
			continue
		}

		return int32(uint32(min) + r%span)
	}
}

// Int64R 生成[min,max]范围内的64位整数
// @param min int64 最小值
// @param max int64 最大值
// @return @1 int64 生成的64位整数
func (rd *Rand) Int64R(min, max int64) int64 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	span := uint64(max) - uint64(min) + 1
	if span == 0 {
		return int64(rd.Uint64())
	}

	limit := ^uint64(0) - (^uint64(0) % span)

	for {
		r := rd.Uint64()
		if r >= limit {
			continue
		}

		return int64(uint64(min) + r%span)
	}
}

// Float32R 生成[min,max)范围内的32位浮点数
// @param min float32 最小值
// @param max float32 最大值
// @return @1 float32 生成的32位浮点数
func (rd *Rand) Float32R(min, max float32) float32 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	return min + rd.Float32()*(max-min)
}

// Float64R 生成[min,max)范围内的64位浮点数
// @param min float64 最小值
// @param max float64 最大值
// @return @1 float64 生成的64位浮点数
func (rd *Rand) Float64R(min, max float64) float64 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	return min + rd.Float64()*(max-min)
}

// Duration 生成[min,max]范围内的时间间隔
// @param min time.Duration 最小时间间隔
// @param max time.Duration 最大时间间隔
// @return @1 time.Duration 生成的时间间隔
func (r *Rand) Duration(min, max time.Duration) time.Duration {
	return time.Duration(r.Int64R(int64(min), int64(max)))
}

// Lucky 根据概率抽取幸运值
// @param probability float64 概率值
// @param base ...float64 可选概率基数，默认为100
// @return @1 bool 是否命中
func (r *Rand) Lucky(probability float64, base ...float64) bool {
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

	return r.Float64() < probability/b
}

// Weight 权重随机，权重合计无效（<=0）时返回 false
// @param fn func(v T) float64 权重计算函数
// @param list ...T 待抽取的元素列表
// @return @1 int 命中元素的下标，未命中时为-1
// @return @2 T 命中的元素，未命中时为零值
// @return @3 bool 是否命中
func (rd *Rand) Weight[T any](fn func(v T) float64, list ...T) (int, T, bool) {
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

	r := rd.Float64() * total
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

// Shuffle 打乱切片
// @param list []T 待打乱的切片
func (r *Rand) Shuffle[T any](list []T) {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	r.rng.Shuffle(len(list), func(i, j int) {
		list[i], list[j] = list[j], list[i]
	})
}
