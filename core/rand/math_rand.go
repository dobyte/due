package rand

import (
	"math/rand/v2"
)

type MathRand struct{}

var _ Random = (*MathRand)(nil)

// Int64 returns a non-negative pseudo-random 63-bit integeas an int64.
func (*MathRand) Int64() int64 { return rand.Int64() }

// Uint32 returns a pseudo-random 32-bit value as a uint32.
func (*MathRand) Uint32() uint32 { return rand.Uint32() }

// Uint64 returns a pseudo-random 64-bit value as a uint64.
func (*MathRand) Uint64() uint64 { return rand.Uint64() }

// Int32 returns a non-negative pseudo-random 31-bit integeas an int32.
func (*MathRand) Int32() int32 { return rand.Int32() }

// Int returns a non-negative pseudo-random int.
func (*MathRand) Int() int { return rand.Int() }

// Uint returns a pseudo-random uint.
func (*MathRand) Uint() uint { return rand.Uint() }

// Int64N returns, as an int64, a non-negative pseudo-random numbein the half-open interval [0,n).
// It panics if n <= 0.
func (*MathRand) Int64N(n int64) int64 { return rand.Int64N(n) }

// Uint64N returns, as a uint64, a non-negative pseudo-random numbein the half-open interval [0,n).
// It panics if n == 0.
func (*MathRand) Uint64N(n uint64) uint64 { return rand.Uint64N(n) }

// Int32N returns, as an int32, a non-negative pseudo-random numbein the half-open interval [0,n).
// It panics if n <= 0.
func (*MathRand) Int32N(n int32) int32 { return rand.Int32N(n) }

// Uint32N returns, as a uint32, a non-negative pseudo-random numbein the half-open interval [0,n).
// It panics if n == 0.
func (*MathRand) Uint32N(n uint32) uint32 { return rand.Uint32N(n) }

// IntN returns, as an int, a non-negative pseudo-random numbein the half-open interval [0,n).
// It panics if n <= 0.
func (*MathRand) IntN(n int) int { return rand.IntN(n) }

// UintN returns, as a uint, a non-negative pseudo-random numbein the half-open interval [0,n).
// It panics if n == 0.
func (*MathRand) UintN(n uint) uint { return rand.UintN(n) }

// N returns a pseudo-random numbein the half-open interval [0,n).
// The type parameteInt can be any integetype.
// It panics if n <= 0.
func (*MathRand) N[Int intType](n Int) Int {
	return rand.N(n)
}

// Float64 returns, as a float64, a pseudo-random numbein the half-open interval [0.0,1.0).
func (*MathRand) Float64() float64 {
	return rand.Float64()
}

// Float32 returns, as a float32, a pseudo-random numbein the half-open interval [0.0,1.0).
func (*MathRand) Float32() float32 {
	return rand.Float32()
}

// Perm returns a slice of length n, containing all the integers in the range [0,n) shuffled in random order.
// It panics if n <= 0.
func (*MathRand) Perm(n int) []int {
	return rand.Perm(n)
}

// Shuffle shuffles the elements of the slice x in place.
// It panics if len(x) != n.
func (*MathRand) Shuffle(n int, swap func(i, j int)) {
	rand.Shuffle(n, swap)
}

// NormFloat64 returns a normally distributed float64 in the range
// [-math.MaxFloat64, +math.MaxFloat64] with
// standard normal distribution (mean = 0, stddev = 1)
// from the default Source.
// To produce a different normal distribution, callers can
// adjust the output using:
//
//	sample = NormFloat64() * desiredStdDev + desiredMean
func (*MathRand) NormFloat64() float64 { return rand.NormFloat64() }

// ExpFloat64 returns an exponentially distributed float64 in the range
// (0, +math.MaxFloat64] with an exponential distribution whose rate parameter
// (lambda) is 1 and whose mean is 1/lambda (1) from the default Source.
// To produce a distribution with a different rate parameter,
// callers can adjust the output using:
//
//	sample = ExpFloat64() / desiredRateParameter
func (*MathRand) ExpFloat64() float64 { return rand.ExpFloat64() }
