package cache

import "github.com/dobyte/due/v2/core/value"

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
