package rand

import (
	"crypto/rand"
	"encoding/binary"
	"math"
)

// TrueRandom 是基于 crypto/rand 实现的真随机数生成器。
// 它没有任何内部状态，底层 crypto/rand.Read 是并发安全的，
// 因此可安全地用于多个 goroutine。
type TrueRandom struct{}

var _ Random = (*TrueRandom)(nil)

// Int 返回一个非负的真随机整数
// @return @1 int 生成的随机整数
func (r *TrueRandom) Int() int { return int(r.readUint64() >> 1) }

// IntN 返回[0,n)范围内的非负真随机整数
// @param n int 范围上限（不包含），n必须大于0
// @return @1 int 生成的随机整数
func (r *TrueRandom) IntN(n int) int {
	if n <= 0 {
		panic("invalid argument to IntN")
	}
	return int(r.uint64n(uint64(n)))
}

// Int32 返回一个非负的真随机31位整数
// @return @1 int32 生成的随机整数
func (r *TrueRandom) Int32() int32 { return int32(r.readUint32() >> 1) }

// Int32N 返回[0,n)范围内的非负真随机32位整数
// @param n int32 范围上限（不包含），n必须大于0
// @return @1 int32 生成的随机整数
func (r *TrueRandom) Int32N(n int32) int32 {
	if n <= 0 {
		panic("invalid argument to Int32N")
	}
	return int32(r.uint32n(uint32(n)))
}

// Int64 返回一个非负的真随机63位整数
// @return @1 int64 生成的随机整数
func (r *TrueRandom) Int64() int64 { return int64(r.readUint64() >> 1) }

// Int64N 返回[0,n)范围内的非负真随机64位整数
// @param n int64 范围上限（不包含），n必须大于0
// @return @1 int64 生成的随机整数
func (r *TrueRandom) Int64N(n int64) int64 {
	if n <= 0 {
		panic("invalid argument to Int64N")
	}
	return int64(r.uint64n(uint64(n)))
}

// Uint 返回一个真随机无符号整数
// @return @1 uint 生成的随机无符号整数
func (r *TrueRandom) Uint() uint { return uint(r.readUint64()) }

// UintN 返回[0,n)范围内的非负真随机无符号整数
// @param n uint 范围上限（不包含），n必须大于0
// @return @1 uint 生成的随机无符号整数
func (r *TrueRandom) UintN(n uint) uint { return uint(r.uint64n(uint64(n))) }

// Uint32 返回一个真随机32位无符号整数
// @return @1 uint32 生成的随机无符号整数
func (r *TrueRandom) Uint32() uint32 { return r.readUint32() }

// Uint32N 返回[0,n)范围内的非负真随机32位无符号整数
// @param n uint32 范围上限（不包含），n必须大于0
// @return @1 uint32 生成的随机无符号整数
func (r *TrueRandom) Uint32N(n uint32) uint32 { return r.uint32n(n) }

// Uint64 返回一个真随机64位无符号整数
// @return @1 uint64 生成的随机无符号整数
func (r *TrueRandom) Uint64() uint64 { return r.readUint64() }

// Uint64N 返回[0,n)范围内的非负真随机64位无符号整数
// @param n uint64 范围上限（不包含），n必须大于0
// @return @1 uint64 生成的随机无符号整数
func (r *TrueRandom) Uint64N(n uint64) uint64 { return r.uint64n(n) }

// N 返回[0,n)范围内的真随机整数，支持任意整数类型
// @param n Int 范围上限（不包含），n必须大于0
// @return @1 Int 生成的随机整数
func (r *TrueRandom) N[Int intType](n Int) Int {
	if n <= 0 {
		panic("invalid argument to N")
	}
	return Int(r.uint64n(uint64(n)))
}

// Float32 返回[0.0,1.0)范围内的真随机32位浮点数
// @return @1 float32 生成的随机浮点数
func (r *TrueRandom) Float32() float32 { return float32(r.readUint32()>>8) / (1 << 24) }

// Float64 返回[0.0,1.0)范围内的真随机64位浮点数
// @return @1 float64 生成的随机浮点数
func (r *TrueRandom) Float64() float64 { return r.uniformFloat64() }

// ExpFloat64 返回服从指数分布的真随机64位浮点数
// 速率参数(lambda)为1，均值为1，使用逆变换法生成
// @return @1 float64 生成的指数分布随机数
func (r *TrueRandom) ExpFloat64() float64 {
	u := r.uniformFloat64()
	for u == 0 {
		u = r.uniformFloat64()
	}
	return -math.Log(u)
}

// NormFloat64 返回服从标准正态分布（均值=0，标准差=1）的真随机64位浮点数
// 使用 Box-Muller 变换将均匀分布转换为标准正态分布
// @return @1 float64 生成的正态分布随机数
func (r *TrueRandom) NormFloat64() float64 {
	u1 := r.uniformFloat64()
	u2 := r.uniformFloat64()
	for u1 == 0 {
		u1 = r.uniformFloat64()
	}
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}

// Perm 返回[0,n)范围内整数的真随机排列切片
// @param n int 排列长度
// @return @1 []int 随机排列后的整数切片
func (r *TrueRandom) Perm(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	r.shuffle(len(p), func(i, j int) {
		p[i], p[j] = p[j], p[i]
	})
	return p
}

// Shuffle 使用交换函数真随机化元素顺序
// @param n int 元素数量，n小于0时会panic
// @param swap func(i, j int) 交换索引i和j位置元素的函数
func (r *TrueRandom) Shuffle(n int, swap func(i, j int)) {
	if n < 0 {
		panic("invalid argument to Shuffle")
	}
	r.shuffle(n, swap)
}

// 从 crypto/rand 读取 8 字节随机数据并转换为 uint64
func (*TrueRandom) readUint64() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return binary.BigEndian.Uint64(b[:])
}

// 从 crypto/rand 读取 4 字节随机数据并转换为 uint32
func (*TrueRandom) readUint32() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return binary.BigEndian.Uint32(b[:])
}

// 返回[0,n)范围内的真随机64位无符号整数
// 使用拒绝采样避免取模偏差，保证均匀分布
func (r *TrueRandom) uint64n(n uint64) uint64 {
	if n == 0 {
		panic("invalid argument to N")
	}

	if n&(n-1) == 0 {
		// n 为 2 的幂时，直接使用掩码即可保证均匀分布
		return r.readUint64() & (n - 1)
	}

	max := ^uint64(0) - (^uint64(0) % n)
	for {
		v := r.readUint64()
		if v < max {
			return v % n
		}
	}
}

// 返回[0,n)范围内的真随机32位无符号整数
// 使用拒绝采样避免取模偏差，保证均匀分布
func (r *TrueRandom) uint32n(n uint32) uint32 {
	if n == 0 {
		panic("invalid argument to N")
	}

	if n&(n-1) == 0 {
		return r.readUint32() & (n - 1)
	}

	max := ^uint32(0) - (^uint32(0) % n)
	for {
		v := r.readUint32()
		if v < max {
			return v % n
		}
	}
}

// 返回[0.0,1.0)范围内的真随机64位浮点数
func (r *TrueRandom) uniformFloat64() float64 {
	return float64(r.readUint64()>>11) / (1 << 53)
}

// 使用 Fisher-Yates 算法打乱元素顺序
func (r *TrueRandom) shuffle(n int, swap func(i, j int)) {
	for i := n - 1; i > 0; i-- {
		j := int(r.uint64n(uint64(i + 1)))
		swap(i, j)
	}
}
