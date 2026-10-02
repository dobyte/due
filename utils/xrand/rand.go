package xrand

import (
	"time"

	"github.com/dobyte/due/v2/core/rand"
)

const (
	LetterSeed           = rand.LetterSeed           // Letters in both upper and lower case
	LetterLowerSeed      = rand.LetterLowerSeed      // Lower-case letters
	LetterUpperSeed      = rand.LetterUpperSeed      // Upper-case letters
	DigitSeed            = rand.DigitSeed            // Digits
	DigitWithoutZeroSeed = rand.DigitWithoutZeroSeed // Digits without zero
	SymbolSeed           = rand.SymbolSeed           // Special symbols
)

// globalRand is the source of random numbers for the top-level
// convenience functions.
var globalRand = rand.NewRand(&rand.PseudoRandom{})

// Uint32 returns a pseudo-random 32-bit unsigned integer.
func Uint32() uint32 {
	return globalRand.Uint32()
}

// Uint64 returns a pseudo-random 64-bit unsigned integer.
func Uint64() uint64 {
	return globalRand.Uint64()
}

// Int32 returns a non-negative pseudo-random 31-bit integer.
func Int32() int32 {
	return globalRand.Int32()
}

// Int returns a non-negative pseudo-random integer.
func Int() int {
	return globalRand.Int()
}

// Uint returns a pseudo-random unsigned integer.
func Uint() uint {
	return globalRand.Uint()
}

// Int64N returns a non-negative pseudo-random 64-bit integer in [0, n). n must be greater than 0.
func Int64N(n int64) int64 {
	return globalRand.Int64N(n)
}

// Uint64N returns a non-negative pseudo-random 64-bit unsigned integer in [0, n). n must be greater
// than 0.
func Uint64N(n uint64) uint64 {
	return globalRand.Uint64N(n)
}

// Int32N returns a non-negative pseudo-random 32-bit integer in [0, n). n must be greater than 0.
func Int32N(n int32) int32 {
	return globalRand.Int32N(n)
}

// Uint32N returns a non-negative pseudo-random 32-bit unsigned integer in [0, n). n must be greater
// than 0.
func Uint32N(n uint32) uint32 {
	return globalRand.Uint32N(n)
}

// IntN returns a non-negative pseudo-random integer in [0, n). n must be greater than 0.
func IntN(n int) int {
	return globalRand.IntN(n)
}

// UintN returns a non-negative pseudo-random unsigned integer in [0, n). n must be greater than 0.
func UintN(n uint) uint {
	return globalRand.UintN(n)
}

// Float64 returns a pseudo-random 64-bit float in [0.0, 1.0).
func Float64() float64 {
	return globalRand.Float64()
}

// Float32 returns a pseudo-random 32-bit float in [0.0, 1.0).
func Float32() float32 {
	return globalRand.Float32()
}

// Perm returns a pseudo-random permutation of the integers in [0, n).
func Perm(n int) []int {
	return globalRand.Perm(n)
}

// ExpFloat64 returns a 64-bit float drawn from an exponential distribution with rate parameter
// (lambda) 1 and mean 1.
func ExpFloat64() float64 {
	return globalRand.ExpFloat64()
}

// NormFloat64 returns a 64-bit float drawn from the standard normal distribution, with mean 0 and
// standard deviation 1.
func NormFloat64() float64 {
	return globalRand.NormFloat64()
}

// Str returns a string of the given length built from the characters of seed.
func Str(seed string, length int) string {
	return globalRand.Str(seed, length)
}

// Letters returns a string of the given length made of letters.
func Letters(length int) string {
	return globalRand.Letters(length)
}

// Digits returns a string of the given length made of digits. The optional hasLeadingZero reports
// whether the string may start with zero.
func Digits(length int, hasLeadingZero ...bool) string {
	return globalRand.Digits(length, hasLeadingZero...)
}

// Symbols returns a string of the given length made of special characters.
func Symbols(length int) string {
	return globalRand.Symbols(length)
}

// IntR returns an integer in [min, max].
func IntR(min, max int) int {
	return globalRand.IntR(min, max)
}

// Int32R returns a 32-bit integer in [min, max].
func Int32R(min, max int32) int32 {
	return globalRand.Int32R(min, max)
}

// Int64R returns a 64-bit integer in [min, max].
func Int64R(min, max int64) int64 {
	return globalRand.Int64R(min, max)
}

// Float32R returns a 32-bit float in [min, max).
func Float32R(min, max float32) float32 {
	return globalRand.Float32R(min, max)
}

// Float64R returns a 64-bit float in [min, max).
func Float64R(min, max float64) float64 {
	return globalRand.Float64R(min, max)
}

// Duration returns a time interval in [min, max].
func Duration(min, max time.Duration) time.Duration {
	return globalRand.Duration(min, max)
}

// Lucky reports whether a lucky draw with the given probability hits. The optional base is the
// probability denominator and defaults to 100.
func Lucky(probability float64, base ...float64) bool {
	return globalRand.Lucky(probability, base...)
}

// Weight performs a weighted random selection over list, where fn returns the weight of an item. It
// returns false when the total weight is invalid (<= 0).
//
// On success it returns the index of the selected item, the item itself and true. When nothing is
// selected it returns -1, the zero value of T and false.
func Weight[T any](fn func(v T) float64, list ...T) (int, T, bool) {
	return globalRand.Weight(fn, list...)
}

// Shuffle randomly reorders list in place.
func Shuffle[T any](list []T) {
	globalRand.Shuffle(list)
}
