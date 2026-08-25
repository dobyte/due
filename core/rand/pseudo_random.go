package rand

import (
	"math/rand/v2"
)

// PseudoRandom 是基于 math/rand/v2 实现的伪随机数生成器。
// 它没有任何内部状态，底层 math/rand/v2 的全局函数是并发安全的，
// 因此可安全地用于多个 goroutine。
type PseudoRandom struct{}

var _ Random = (*PseudoRandom)(nil)

// Int 返回一个非负的伪随机整数
// @return @1 int 生成的随机整数
func (*PseudoRandom) Int() int { return rand.Int() }

// IntN 返回[0,n)范围内的非负伪随机整数
// @param n int 范围上限（不包含），n必须大于0
// @return @1 int 生成的随机整数
func (*PseudoRandom) IntN(n int) int { return rand.IntN(n) }

// Int32 返回一个非负的伪随机31位整数
// @return @1 int32 生成的随机整数
func (*PseudoRandom) Int32() int32 { return rand.Int32() }

// Int32N 返回[0,n)范围内的非负伪随机32位整数
// @param n int32 范围上限（不包含），n必须大于0
// @return @1 int32 生成的随机整数
func (*PseudoRandom) Int32N(n int32) int32 { return rand.Int32N(n) }

// Int64 返回一个非负的伪随机63位整数
// @return @1 int64 生成的随机整数
func (*PseudoRandom) Int64() int64 { return rand.Int64() }

// Int64N 返回[0,n)范围内的非负伪随机64位整数
// @param n int64 范围上限（不包含），n必须大于0
// @return @1 int64 生成的随机整数
func (*PseudoRandom) Int64N(n int64) int64 { return rand.Int64N(n) }

// Uint 返回一个伪随机无符号整数
// @return @1 uint 生成的随机无符号整数
func (*PseudoRandom) Uint() uint { return rand.Uint() }

// UintN 返回[0,n)范围内的非负伪随机无符号整数
// @param n uint 范围上限（不包含），n必须大于0
// @return @1 uint 生成的随机无符号整数
func (*PseudoRandom) UintN(n uint) uint { return rand.UintN(n) }

// Uint32 返回一个伪随机32位无符号整数
// @return @1 uint32 生成的随机无符号整数
func (*PseudoRandom) Uint32() uint32 { return rand.Uint32() }

// Uint32N 返回[0,n)范围内的非负伪随机32位无符号整数
// @param n uint32 范围上限（不包含），n必须大于0
// @return @1 uint32 生成的随机无符号整数
func (*PseudoRandom) Uint32N(n uint32) uint32 { return rand.Uint32N(n) }

// Uint64 返回一个伪随机64位无符号整数
// @return @1 uint64 生成的随机无符号整数
func (*PseudoRandom) Uint64() uint64 { return rand.Uint64() }

// Uint64N 返回[0,n)范围内的非负伪随机64位无符号整数
// @param n uint64 范围上限（不包含），n必须大于0
// @return @1 uint64 生成的随机无符号整数
func (*PseudoRandom) Uint64N(n uint64) uint64 { return rand.Uint64N(n) }

// N 返回[0,n)范围内的伪随机整数，支持任意整数类型
// @param n Int 范围上限（不包含），n必须大于0
// @return @1 Int 生成的随机整数
func (*PseudoRandom) N[Int intType](n Int) Int {
	return rand.N(n)
}

// Float32 返回[0.0,1.0)范围内的伪随机32位浮点数
// @return @1 float32 生成的随机浮点数
func (*PseudoRandom) Float32() float32 {
	return rand.Float32()
}

// Float64 返回[0.0,1.0)范围内的伪随机64位浮点数
// @return @1 float64 生成的随机浮点数
func (*PseudoRandom) Float64() float64 {
	return rand.Float64()
}

// ExpFloat64 返回服从指数分布的伪随机64位浮点数
// 速率参数(lambda)为1，均值为1
// @return @1 float64 生成的指数分布随机数
func (*PseudoRandom) ExpFloat64() float64 { return rand.ExpFloat64() }

// NormFloat64 返回服从标准正态分布（均值=0，标准差=1）的伪随机64位浮点数
// @return @1 float64 生成的正态分布随机数
func (*PseudoRandom) NormFloat64() float64 { return rand.NormFloat64() }

// Perm 返回[0,n)范围内整数的伪随机排列切片
// @param n int 排列长度
// @return @1 []int 随机排列后的整数切片
func (*PseudoRandom) Perm(n int) []int {
	return rand.Perm(n)
}

// Shuffle 使用交换函数伪随机化元素顺序
// @param n int 元素数量，n小于0时会panic
// @param swap func(i, j int) 交换索引i和j位置元素的函数
func (*PseudoRandom) Shuffle(n int, swap func(i, j int)) {
	rand.Shuffle(n, swap)
}
