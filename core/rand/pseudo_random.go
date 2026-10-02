package rand

import (
	"math/rand/v2"
)

// PseudoRandom is a pseudo-random number generator implemented on top of math/rand/v2.
//
// It keeps no internal state and the global functions of math/rand/v2 are safe for concurrent use,
// so a PseudoRandom is safe to share across goroutines.
type PseudoRandom struct{}

var _ Random = (*PseudoRandom)(nil)

// Int returns a non-negative pseudo-random integer.
func (*PseudoRandom) Int() int { return rand.Int() }

// IntN returns a non-negative pseudo-random integer in [0,n). n must be greater than 0.
func (*PseudoRandom) IntN(n int) int { return rand.IntN(n) }

// Int32 returns a non-negative pseudo-random 31-bit integer.
func (*PseudoRandom) Int32() int32 { return rand.Int32() }

// Int32N returns a non-negative pseudo-random 32-bit integer in [0,n). n must be greater than 0.
func (*PseudoRandom) Int32N(n int32) int32 { return rand.Int32N(n) }

// Int64 returns a non-negative pseudo-random 63-bit integer.
func (*PseudoRandom) Int64() int64 { return rand.Int64() }

// Int64N returns a non-negative pseudo-random 64-bit integer in [0,n). n must be greater than 0.
func (*PseudoRandom) Int64N(n int64) int64 { return rand.Int64N(n) }

// Uint returns a pseudo-random unsigned integer.
func (*PseudoRandom) Uint() uint { return rand.Uint() }

// UintN returns a non-negative pseudo-random unsigned integer in [0,n). n must be greater than 0.
func (*PseudoRandom) UintN(n uint) uint { return rand.UintN(n) }

// Uint32 returns a pseudo-random 32-bit unsigned integer.
func (*PseudoRandom) Uint32() uint32 { return rand.Uint32() }

// Uint32N returns a non-negative pseudo-random 32-bit unsigned integer in [0,n). n must be greater
// than 0.
func (*PseudoRandom) Uint32N(n uint32) uint32 { return rand.Uint32N(n) }

// Uint64 returns a pseudo-random 64-bit unsigned integer.
func (*PseudoRandom) Uint64() uint64 { return rand.Uint64() }

// Uint64N returns a non-negative pseudo-random 64-bit unsigned integer in [0,n). n must be greater
// than 0.
func (*PseudoRandom) Uint64N(n uint64) uint64 { return rand.Uint64N(n) }

// N returns a pseudo-random integer in [0,n) for any integer type. n must be greater than 0.
func (*PseudoRandom) N[Int intType](n Int) Int {
	return rand.N(n)
}

// Float32 returns a pseudo-random 32-bit float in [0.0,1.0).
func (*PseudoRandom) Float32() float32 {
	return rand.Float32()
}

// Float64 returns a pseudo-random 64-bit float in [0.0,1.0).
func (*PseudoRandom) Float64() float64 {
	return rand.Float64()
}

// ExpFloat64 returns a pseudo-random 64-bit float from an exponential distribution with rate
// parameter (lambda) 1 and mean 1.
func (*PseudoRandom) ExpFloat64() float64 { return rand.ExpFloat64() }

// NormFloat64 returns a pseudo-random 64-bit float from the standard normal distribution (mean 0,
// standard deviation 1).
func (*PseudoRandom) NormFloat64() float64 { return rand.NormFloat64() }

// Perm returns a pseudo-random permutation of the integers in [0,n).
func (*PseudoRandom) Perm(n int) []int {
	return rand.Perm(n)
}

// Shuffle randomizes the order of the elements using swap. n is the number of elements and must
// not be negative.
func (*PseudoRandom) Shuffle(n int, swap func(i, j int)) {
	rand.Shuffle(n, swap)
}
