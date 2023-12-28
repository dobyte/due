package cache

import (
	"github.com/dobyte/due/v2/core/value"
	"time"
)

type Result interface {
}

type result struct {
	err   error
	value value.Value
}

func NewResult(val interface{}, err ...error) Result {
	if len(err) > 0 {
		return &result{err: err[0], value: value.NewValue(val)}
	} else {
		return &result{value: value.NewValue(val)}
	}
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

func (r *result) Slice() ([]interface{}, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Slice(), nil
}

func (r *result) Map() (map[string]interface{}, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.value.Map(), nil
}

func (r *result) Scan(pointer interface{}) error {
	if r.err != nil {
		return r.err
	}

	return r.value.Scan(pointer)
}

func (r *result) Value() (interface{}, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.Value(), nil
}
