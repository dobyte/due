package rand

import (
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/log"
)

const (
	LetterSeed           = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" // Upper- and lower-case letters
	LetterLowerSeed      = "abcdefghijklmnopqrstuvwxyz"                           // Lower-case letters
	LetterUpperSeed      = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"                           // Upper-case letters
	DigitSeed            = "0123456789"                                           // Digits
	DigitWithoutZeroSeed = "123456789"                                            // Digits without zero
	SymbolSeed           = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"                   // Special characters
)

type intType interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

type Rand struct {
	mu  *sync.Mutex
	rng Random
}

// NewRand returns a new random number generator backed by rng.
//
// Passing true for safe enables concurrency safety by guarding rng with a mutex.
func NewRand(rng Random, safe ...bool) *Rand {
	rd := &Rand{}
	rd.rng = rng
	if len(safe) > 0 && safe[0] {
		rd.mu = &sync.Mutex{}
	}

	return rd
}

// Int64 returns a non-negative pseudo-random 63-bit integer.
func (r *Rand) Int64() int64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Int64()
}

// Uint32 returns a pseudo-random 32-bit unsigned integer.
func (r *Rand) Uint32() uint32 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Uint32()
}

// Uint64 returns a pseudo-random 64-bit unsigned integer.
func (r *Rand) Uint64() uint64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Uint64()
}

// Int32 returns a non-negative pseudo-random 31-bit integer.
func (r *Rand) Int32() int32 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Int32()
}

// Int returns a non-negative pseudo-random integer.
func (r *Rand) Int() int {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Int()
}

// Uint returns a pseudo-random unsigned integer.
func (r *Rand) Uint() uint {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Uint()
}

// Int64N returns a non-negative pseudo-random 64-bit integer in [0,n). n must be greater than 0.
func (r *Rand) Int64N(n int64) int64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Int64N(n)
}

// Uint64N returns a non-negative pseudo-random 64-bit unsigned integer in [0,n). n must be
// greater than 0.
func (r *Rand) Uint64N(n uint64) uint64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Uint64N(n)
}

// Int32N returns a non-negative pseudo-random 32-bit integer in [0,n). n must be greater than 0.
func (r *Rand) Int32N(n int32) int32 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Int32N(n)
}

// Uint32N returns a non-negative pseudo-random 32-bit unsigned integer in [0,n). n must be
// greater than 0.
func (r *Rand) Uint32N(n uint32) uint32 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Uint32N(n)
}

// IntN returns a non-negative pseudo-random integer in [0,n). n must be greater than 0.
func (r *Rand) IntN(n int) int {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.IntN(n)
}

// UintN returns a non-negative pseudo-random unsigned integer in [0,n). n must be greater than 0.
func (r *Rand) UintN(n uint) uint {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.UintN(n)
}

// Float64 returns a pseudo-random 64-bit float in [0.0,1.0).
func (r *Rand) Float64() float64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Float64()
}

// Float32 returns a pseudo-random 32-bit float in [0.0,1.0).
func (r *Rand) Float32() float32 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Float32()
}

// Perm returns a pseudo-random permutation of the integers in [0,n).
func (r *Rand) Perm(n int) []int {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.Perm(n)
}

// ExpFloat64 returns a 64-bit float from an exponential distribution with rate parameter
// (lambda) 1 and mean 1.
func (r *Rand) ExpFloat64() float64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.ExpFloat64()
}

// NormFloat64 returns a 64-bit float from the standard normal distribution (mean 0, standard
// deviation 1).
func (r *Rand) NormFloat64() float64 {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	return r.rng.NormFloat64()
}

// Str returns a random string of the given length built from seed.
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

// Letters returns a random string of the given length built from letters.
func (r *Rand) Letters(length int) string {
	return r.Str(LetterSeed, length)
}

// Digits returns a random digit string of the given length. By default the first digit is not zero
// unless hasLeadingZero is true.
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

// Symbols returns a random string of the given length built from special characters.
func (r *Rand) Symbols(length int) string {
	return r.Str(SymbolSeed, length)
}

// IntR returns a pseudo-random integer in [min,max].
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

	// Rejection sampling avoids the panic caused by max+1 overflow and the modulo bias.
	limit := ^uint64(0) - (^uint64(0) % span)

	for {
		r := rd.Uint64()
		if r >= limit {
			continue
		}

		return int(uint64(min) + r%span)
	}
}

// Int32R returns a pseudo-random 32-bit integer in [min,max].
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

// Int64R returns a pseudo-random 64-bit integer in [min,max].
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

// Float32R returns a pseudo-random 32-bit float in [min,max).
func (rd *Rand) Float32R(min, max float32) float32 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	return min + rd.Float32()*(max-min)
}

// Float64R returns a pseudo-random 64-bit float in [min,max).
func (rd *Rand) Float64R(min, max float64) float64 {
	if min == max {
		return min
	}

	if min > max {
		min, max = max, min
	}

	return min + rd.Float64()*(max-min)
}

// Duration returns a pseudo-random duration in [min,max].
func (r *Rand) Duration(min, max time.Duration) time.Duration {
	return time.Duration(r.Int64R(int64(min), int64(max)))
}

// Lucky reports whether a random draw hits the given probability.
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

// Weight performs a weighted random draw over list, using fn to compute each element's weight.
//
// It returns the index of the chosen element, the element itself and true, or -1, the zero value
// and false when no element is drawn. A draw fails when the total weight is not positive.
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

// Shuffle shuffles list in place.
func (r *Rand) Shuffle[T any](list []T) {
	if r.mu != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	}

	r.rng.Shuffle(len(list), func(i, j int) {
		list[i], list[j] = list[j], list[i]
	})
}
