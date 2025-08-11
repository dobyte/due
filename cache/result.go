package cache

import (
	"time"

	"github.com/dobyte/due/v2/core/value"
)

type Result interface {
	Err() error
	Result() (value.Value, error)
	Int() (int, error)
	Int8() (int8, error)
	Int16() (int16, error)
	Int32() (int32, error)
	Int64() (int64, error)
	Uint() (uint, error)
	Uint8() (uint8, error)
	Uint16() (uint16, error)
	Uint32() (uint32, error)
	Uint64() (uint64, error)
	Float32() (float32, error)
	Float64() (float64, error)
	Bool() (bool, error)
	String() (string, error)
	Duration() (time.Duration, error)
	Ints() ([]int, error)
	Int8s() ([]int8, error)
	Int16s() ([]int16, error)
	Int32s() ([]int32, error)
	Int64s() ([]int64, error)
	Uints() ([]uint, error)
	Uint8s() ([]uint8, error)
	Uint16s() ([]uint16, error)
	Uint32s() ([]uint32, error)
	Uint64s() ([]uint64, error)
	Float32s() ([]float32, error)
	Float64s() ([]float64, error)
	Bools() ([]bool, error)
	Strings() ([]string, error)
	Bytes() ([]byte, error)
	Durations() ([]time.Duration, error)
	Slice() ([]any, error)
	Map() (map[string]any, error)
	Scan(pointer any) error
}

type result struct {
	err   error
	value value.Value
}

func NewResult(val any, err ...error) Result {
	if len(err) > 0 {
		return &result{err: err[0], value: value.NewValue(val)}
	} else {
		return &result{value: value.NewValue(val)}
	}
}

func (r *result) Err() error {
	return r.err
}

func (r *result) Result() (value.Value, error) {
	return r.value, r.err
}

func (r *result) Int() (int, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int(), nil
}

func (r *result) Int8() (int8, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int8(), nil
}

func (r *result) Int16() (int16, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int16(), nil
}

func (r *result) Int32() (int32, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int32(), nil
}

func (r *result) Int64() (int64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Int64(), nil
}

func (r *result) Uint() (uint, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint(), nil
}

func (r *result) Uint8() (uint8, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint8(), nil
}

func (r *result) Uint16() (uint16, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint16(), nil
}

func (r *result) Uint32() (uint32, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint32(), nil
}

func (r *result) Uint64() (uint64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Uint64(), nil
}

func (r *result) Float32() (float32, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Float32(), nil
}

func (r *result) Float64() (float64, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Float64(), nil
}

func (r *result) Bool() (bool, error) {
	if r.err != nil {
		return false, r.err
	}

	return r.value.Bool(), nil
}

func (r *result) String() (string, error) {
	if r.err != nil {
		return "", r.err
	}

	return r.value.String(), nil
}

func (r *result) Duration() (time.Duration, error) {
	if r.err != nil {
		return 0, r.err
	}

	return r.value.Duration(), nil
}

func (r *result) Ints() ([]int, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Ints(), nil
}

func (r *result) Int8s() ([]int8, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int8s(), nil
}

func (r *result) Int16s() ([]int16, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int16s(), nil
}

func (r *result) Int32s() ([]int32, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int32s(), nil
}

func (r *result) Int64s() ([]int64, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Int64s(), nil
}

func (r *result) Uints() ([]uint, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uints(), nil
}

func (r *result) Uint8s() ([]uint8, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint8s(), nil
}

func (r *result) Uint16s() ([]uint16, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint16s(), nil
}

func (r *result) Uint32s() ([]uint32, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint32s(), nil
}

func (r *result) Uint64s() ([]uint64, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Uint64s(), nil
}

func (r *result) Float32s() ([]float32, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Float32s(), nil
}

func (r *result) Float64s() ([]float64, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Float64s(), nil
}

func (r *result) Bools() ([]bool, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Bools(), nil
}

func (r *result) Strings() ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Strings(), nil
}

func (r *result) Bytes() ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Bytes(), nil
}

func (r *result) Durations() ([]time.Duration, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Durations(), nil
}

func (r *result) Slice() ([]any, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Slice(), nil
}

func (r *result) Map() (map[string]any, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Map(), nil
}

func (r *result) Scan(pointer any) error {
	if r.err != nil {
		return r.err
	}

	return r.value.Scan(pointer)
}
