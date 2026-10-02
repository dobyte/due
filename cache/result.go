package cache

import (
	"time"

	"github.com/dobyte/due/v2/core/value"
)

// Result is the interface that represents a cache read result. It provides methods to convert the
// cached value into the supported types.
type Result interface {
	// Err returns the error of the result, if any.
	Err() error
	// Result returns the raw value together with the error, if any.
	Result() (value.Value, error)
	// Int returns the cached value as an int.
	Int() (int, error)
	// Int8 returns the cached value as an int8.
	Int8() (int8, error)
	// Int16 returns the cached value as an int16.
	Int16() (int16, error)
	// Int32 returns the cached value as an int32.
	Int32() (int32, error)
	// Int64 returns the cached value as an int64.
	Int64() (int64, error)
	// Uint returns the cached value as a uint.
	Uint() (uint, error)
	// Uint8 returns the cached value as a uint8.
	Uint8() (uint8, error)
	// Uint16 returns the cached value as a uint16.
	Uint16() (uint16, error)
	// Uint32 returns the cached value as a uint32.
	Uint32() (uint32, error)
	// Uint64 returns the cached value as a uint64.
	Uint64() (uint64, error)
	// Float32 returns the cached value as a float32.
	Float32() (float32, error)
	// Float64 returns the cached value as a float64.
	Float64() (float64, error)
	// Bool returns the cached value as a bool.
	Bool() (bool, error)
	// String returns the cached value as a string.
	String() (string, error)
	// Duration returns the cached value as a time.Duration.
	Duration() (time.Duration, error)
	// Ints returns the cached value as a []int.
	Ints() ([]int, error)
	// Int8s returns the cached value as a []int8.
	Int8s() ([]int8, error)
	// Int16s returns the cached value as a []int16.
	Int16s() ([]int16, error)
	// Int32s returns the cached value as a []int32.
	Int32s() ([]int32, error)
	// Int64s returns the cached value as a []int64.
	Int64s() ([]int64, error)
	// Uints returns the cached value as a []uint.
	Uints() ([]uint, error)
	// Uint8s returns the cached value as a []uint8.
	Uint8s() ([]uint8, error)
	// Uint16s returns the cached value as a []uint16.
	Uint16s() ([]uint16, error)
	// Uint32s returns the cached value as a []uint32.
	Uint32s() ([]uint32, error)
	// Uint64s returns the cached value as a []uint64.
	Uint64s() ([]uint64, error)
	// Float32s returns the cached value as a []float32.
	Float32s() ([]float32, error)
	// Float64s returns the cached value as a []float64.
	Float64s() ([]float64, error)
	// Bools returns the cached value as a []bool.
	Bools() ([]bool, error)
	// Strings returns the cached value as a []string.
	Strings() ([]string, error)
	// Bytes returns the cached value as a []byte.
	Bytes() ([]byte, error)
	// Durations returns the cached value as a []time.Duration.
	Durations() ([]time.Duration, error)
	// Slice returns the cached value as a []any.
	Slice() ([]any, error)
	// Map returns the cached value as a map[string]any.
	Map() (map[string]any, error)
	// Scan scans the cached value into the object pointed to by pointer.
	Scan(pointer any) error
}

// result is the default implementation of Result.
type result struct {
	err   error
	value value.Value
}

// NewResult returns a new Result that holds val and the optional error.
func NewResult(val any, err ...error) Result {
	if len(err) > 0 {
		return &result{err: err[0], value: value.NewValue(val)}
	} else {
		return &result{value: value.NewValue(val)}
	}
}

// Err returns the error of the result, if any.
func (r *result) Err() error {
	return r.err
}

// Result returns the raw value together with the error, if any.
func (r *result) Result() (value.Value, error) {
	return r.value, r.err
}

// Int returns the cached value as an int.
func (r *result) Int() (int, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int(), nil
}

// Int8 returns the cached value as an int8.
func (r *result) Int8() (int8, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int8(), nil
}

// Int16 returns the cached value as an int16.
func (r *result) Int16() (int16, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int16(), nil
}

// Int32 returns the cached value as an int32.
func (r *result) Int32() (int32, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int32(), nil
}

// Int64 returns the cached value as an int64.
func (r *result) Int64() (int64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int64(), nil
}

// Uint returns the cached value as a uint.
func (r *result) Uint() (uint, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint(), nil
}

// Uint8 returns the cached value as a uint8.
func (r *result) Uint8() (uint8, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint8(), nil
}

// Uint16 returns the cached value as a uint16.
func (r *result) Uint16() (uint16, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint16(), nil
}

// Uint32 returns the cached value as a uint32.
func (r *result) Uint32() (uint32, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint32(), nil
}

// Uint64 returns the cached value as a uint64.
func (r *result) Uint64() (uint64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint64(), nil
}

// Float32 returns the cached value as a float32.
func (r *result) Float32() (float32, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Float32(), nil
}

// Float64 returns the cached value as a float64.
func (r *result) Float64() (float64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Float64(), nil
}

// Rune returns the cached value as a rune.
func (r *result) Rune() (rune, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Rune(), nil
}

// Bool returns the cached value as a bool.
func (r *result) Bool() (bool, error) {
	if r.err != nil {
		return false, r.err
	}

	return r.value.Bool(), nil
}

// String returns the cached value as a string.
func (r *result) String() (string, error) {
	if r.err != nil {
		return "", r.err
	}

	return r.value.String(), nil
}

// B returns the cached value as a float64.
func (r *result) B() (float64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.B(), nil
}

// Duration returns the cached value as a time.Duration.
func (r *result) Duration() (time.Duration, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Duration(), nil
}

// Ints returns the cached value as a []int.
func (r *result) Ints() ([]int, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Ints(), nil
}

// Int8s returns the cached value as a []int8.
func (r *result) Int8s() ([]int8, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int8s(), nil
}

// Int16s returns the cached value as a []int16.
func (r *result) Int16s() ([]int16, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int16s(), nil
}

// Int32s returns the cached value as a []int32.
func (r *result) Int32s() ([]int32, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int32s(), nil
}

// Int64s returns the cached value as a []int64.
func (r *result) Int64s() ([]int64, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int64s(), nil
}

// Uints returns the cached value as a []uint.
func (r *result) Uints() ([]uint, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uints(), nil
}

// Uint8s returns the cached value as a []uint8.
func (r *result) Uint8s() ([]uint8, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint8s(), nil
}

// Uint16s returns the cached value as a []uint16.
func (r *result) Uint16s() ([]uint16, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint16s(), nil
}

// Uint32s returns the cached value as a []uint32.
func (r *result) Uint32s() ([]uint32, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint32s(), nil
}

// Uint64s returns the cached value as a []uint64.
func (r *result) Uint64s() ([]uint64, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint64s(), nil
}

// Float32s returns the cached value as a []float32.
func (r *result) Float32s() ([]float32, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Float32s(), nil
}

// Float64s returns the cached value as a []float64.
func (r *result) Float64s() ([]float64, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Float64s(), nil
}

// Runes returns the cached value as a []rune.
func (r *result) Runes() ([]rune, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Runes(), nil
}

// Bools returns the cached value as a []bool.
func (r *result) Bools() ([]bool, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Bools(), nil
}

// Strings returns the cached value as a []string.
func (r *result) Strings() ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Strings(), nil
}

// Bs returns the cached value as a []float64.
func (r *result) Bs() ([]float64, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Bs(), nil
}

// Bytes returns the cached value as a []byte.
func (r *result) Bytes() ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Bytes(), nil
}

// Durations returns the cached value as a []time.Duration.
func (r *result) Durations() ([]time.Duration, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Durations(), nil
}

// Slice returns the cached value as a []any.
func (r *result) Slice() ([]any, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Slice(), nil
}

// Map returns the cached value as a map[string]any.
func (r *result) Map() (map[string]any, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Map(), nil
}

// Scan scans the cached value into the object pointed to by pointer.
func (r *result) Scan(pointer any) error {
	if r.err != nil {
		return r.err
	}

	return r.value.Scan(pointer)
}
