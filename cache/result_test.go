package cache

import (
	"testing"

	"github.com/dobyte/due/v2/errors"
)

// resultCall wraps a Result method so that its error path can be exercised uniformly.
type resultCall struct {
	name string
	call func(r Result) (any, error)
}

// resultCalls returns one entry per method declared on the Result interface.
func resultCalls() []resultCall {
	return []resultCall{
		{"Err", func(r Result) (any, error) { return nil, r.Err() }},
		{"Result", func(r Result) (any, error) { return r.Result() }},
		{"Int", func(r Result) (any, error) { return r.Int() }},
		{"Int8", func(r Result) (any, error) { return r.Int8() }},
		{"Int16", func(r Result) (any, error) { return r.Int16() }},
		{"Int32", func(r Result) (any, error) { return r.Int32() }},
		{"Int64", func(r Result) (any, error) { return r.Int64() }},
		{"Uint", func(r Result) (any, error) { return r.Uint() }},
		{"Uint8", func(r Result) (any, error) { return r.Uint8() }},
		{"Uint16", func(r Result) (any, error) { return r.Uint16() }},
		{"Uint32", func(r Result) (any, error) { return r.Uint32() }},
		{"Uint64", func(r Result) (any, error) { return r.Uint64() }},
		{"Float32", func(r Result) (any, error) { return r.Float32() }},
		{"Float64", func(r Result) (any, error) { return r.Float64() }},
		{"Bool", func(r Result) (any, error) { return r.Bool() }},
		{"String", func(r Result) (any, error) { return r.String() }},
		{"Duration", func(r Result) (any, error) { return r.Duration() }},
		{"Ints", func(r Result) (any, error) { return r.Ints() }},
		{"Int8s", func(r Result) (any, error) { return r.Int8s() }},
		{"Int16s", func(r Result) (any, error) { return r.Int16s() }},
		{"Int32s", func(r Result) (any, error) { return r.Int32s() }},
		{"Int64s", func(r Result) (any, error) { return r.Int64s() }},
		{"Uints", func(r Result) (any, error) { return r.Uints() }},
		{"Uint8s", func(r Result) (any, error) { return r.Uint8s() }},
		{"Uint16s", func(r Result) (any, error) { return r.Uint16s() }},
		{"Uint32s", func(r Result) (any, error) { return r.Uint32s() }},
		{"Uint64s", func(r Result) (any, error) { return r.Uint64s() }},
		{"Float32s", func(r Result) (any, error) { return r.Float32s() }},
		{"Float64s", func(r Result) (any, error) { return r.Float64s() }},
		{"Bools", func(r Result) (any, error) { return r.Bools() }},
		{"Strings", func(r Result) (any, error) { return r.Strings() }},
		{"Bytes", func(r Result) (any, error) { return r.Bytes() }},
		{"Durations", func(r Result) (any, error) { return r.Durations() }},
		{"Slice", func(r Result) (any, error) { return r.Slice() }},
		{"Map", func(r Result) (any, error) { return r.Map() }},
		{"Scan", func(r Result) (any, error) {
			var v int
			err := r.Scan(&v)
			return v, err
		}},
	}
}

// extraResult exposes the conversion methods that are implemented on *result but are not part of
// the Result interface.
type extraResult interface {
	Rune() (rune, error)
	B() (float64, error)
	Runes() ([]rune, error)
	Bs() ([]float64, error)
}

// TestResultErrorPropagation verifies that every conversion method returns the stored error.
func TestResultErrorPropagation(t *testing.T) {
	want := errors.ErrMissingCacheInstance
	r := NewResult("1", want)

	if r.Err() != want {
		t.Errorf("Err() = %v, want %v", r.Err(), want)
	}

	for _, c := range resultCalls() {
		t.Run(c.name, func(t *testing.T) {
			if _, err := c.call(r); err != want {
				t.Errorf("%s error = %v, want %v", c.name, err, want)
			}
		})
	}

	extra, ok := r.(extraResult)
	if !ok {
		t.Fatal("result must implement the extra conversion methods")
	}

	t.Run("Rune", func(t *testing.T) {
		if _, err := extra.Rune(); err != want {
			t.Errorf("Rune error = %v, want %v", err, want)
		}
	})
	t.Run("B", func(t *testing.T) {
		if _, err := extra.B(); err != want {
			t.Errorf("B error = %v, want %v", err, want)
		}
	})
	t.Run("Runes", func(t *testing.T) {
		if _, err := extra.Runes(); err != want {
			t.Errorf("Runes error = %v, want %v", err, want)
		}
	})
	t.Run("Bs", func(t *testing.T) {
		if _, err := extra.Bs(); err != want {
			t.Errorf("Bs error = %v, want %v", err, want)
		}
	})
}

// TestResultValueConversion verifies that every conversion method succeeds when no error is held.
func TestResultValueConversion(t *testing.T) {
	r := NewResult("123")

	if r.Err() != nil {
		t.Fatalf("Err() = %v, want nil", r.Err())
	}

	for _, c := range resultCalls() {
		t.Run(c.name, func(t *testing.T) {
			if _, err := c.call(r); err != nil {
				t.Errorf("%s error = %v, want nil", c.name, err)
			}
		})
	}

	extra, ok := r.(extraResult)
	if !ok {
		t.Fatal("result must implement the extra conversion methods")
	}

	t.Run("Rune", func(t *testing.T) {
		if _, err := extra.Rune(); err != nil {
			t.Errorf("Rune error = %v, want nil", err)
		}
	})
	t.Run("B", func(t *testing.T) {
		if _, err := extra.B(); err != nil {
			t.Errorf("B error = %v, want nil", err)
		}
	})
	t.Run("Runes", func(t *testing.T) {
		if _, err := extra.Runes(); err != nil {
			t.Errorf("Runes error = %v, want nil", err)
		}
	})
	t.Run("Bs", func(t *testing.T) {
		if _, err := extra.Bs(); err != nil {
			t.Errorf("Bs error = %v, want nil", err)
		}
	})
}

// TestNewResult verifies the construction of a result with and without an error.
func TestNewResult(t *testing.T) {
	t.Run("without error", func(t *testing.T) {
		r := NewResult("123")
		if r.Err() != nil {
			t.Errorf("Err() = %v, want nil", r.Err())
		}
	})

	t.Run("with error", func(t *testing.T) {
		want := errors.ErrMissingCacheInstance
		r := NewResult("123", want)
		if r.Err() != want {
			t.Errorf("Err() = %v, want %v", r.Err(), want)
		}
	})
}

// TestResultConversions verifies a few concrete conversion results to keep the assertions
// meaningful rather than merely error free.
func TestResultConversions(t *testing.T) {
	r := NewResult("123")

	if v, err := r.Int(); err != nil || v != 123 {
		t.Errorf("Int() = (%d, %v), want (123, nil)", v, err)
	}
	if v, err := r.Int64(); err != nil || v != 123 {
		t.Errorf("Int64() = (%d, %v), want (123, nil)", v, err)
	}
	if v, err := r.String(); err != nil || v != "123" {
		t.Errorf("String() = (%q, %v), want (%q, nil)", v, err, "123")
	}
	if v, err := r.Bytes(); err != nil || string(v) != "123" {
		t.Errorf("Bytes() = (%q, %v), want (%q, nil)", v, err, "123")
	}
	if _, err := r.Duration(); err != nil {
		t.Errorf("Duration() error = %v, want nil", err)
	}

	raw, err := r.Result()
	if err != nil {
		t.Fatalf("Result() error = %v, want nil", err)
	}
	if raw == nil {
		t.Error("Result() must return a non-nil value")
	}
	if raw.Value() != "123" {
		t.Errorf("Result() value = %v, want %q", raw.Value(), "123")
	}

	var scanned string
	if err := r.Scan(&scanned); err != nil || scanned != "123" {
		t.Errorf("Scan() = (%q, %v), want (%q, nil)", scanned, err, "123")
	}
}
