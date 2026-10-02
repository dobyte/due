package xconv_test

import (
	"slices"
	"testing"

	"github.com/dobyte/due/v2/utils/xconv"
)

// exInts is a named slice type used to exercise the reflection slice fallback
// of the slice converters.
type exInts []int

// exNumber is the set of numeric target kinds exercised by the exhaustive
// slice conversion tests.
type exNumber interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// exLevel describes a single exhaustive slice conversion input. want holds the
// logical element values for numeric targets, while wantFloat optionally holds
// a floating-point specific expectation (used by [][]byte, whose elements
// convert to zero for floating-point targets).
type exLevel struct {
	name      string
	in        any
	want      []int64
	wantFloat []int64
}

// floatWant returns the expected logical values for floating-point targets,
// falling back to want when no floating-point specific expectation is set.
func (c exLevel) floatWant() []int64 {
	if c.wantFloat != nil {
		return c.wantFloat
	}
	return c.want
}

// exSlicePtr returns a pointer to a copy of v so pointer inputs can be built inline.
func exSlicePtr[T any](v T) *T {
	return &v
}

// exTo converts logical element values to the concrete numeric target type,
// preserving nil so nil inputs can be compared against nil results.
func exTo[T exNumber](w []int64) []T {
	if w == nil {
		return nil
	}

	out := make([]T, len(w))
	for i := range w {
		out[i] = T(w[i])
	}
	return out
}

// exAssertSlice compares got against want and reports a formatted failure.
func exAssertSlice[T exNumber](t *testing.T, fn string, in any, got, want []T) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Errorf("%s(%v) = %v, want %v", fn, in, got, want)
	}
}

// exSliceCorpus returns the exhaustive input table shared by every slice
// converter. It feeds one slice (and pointer to slice) per supported element
// type, the reflection fallbacks for named slices and arrays, nil pointer
// values and non-slice inputs.
func exSliceCorpus() []exLevel {
	return []exLevel{
		// Nil and non-slice inputs.
		{name: "nil", in: nil, want: nil},
		{name: "non-slice int", in: 1, want: nil},
		{name: "non-slice string", in: "x", want: nil},
		{name: "non-slice struct", in: struct{}{}, want: nil},

		// Slices.
		{name: "int", in: []int{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "int8", in: []int8{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "int16", in: []int16{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "int32", in: []int32{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "int64", in: []int64{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "uint", in: []uint{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "uint8", in: []uint8{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "uint16", in: []uint16{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "uint32", in: []uint32{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "uint64", in: []uint64{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "float32", in: []float32{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "float64", in: []float64{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "complex64", in: []complex64{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "complex128", in: []complex128{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "string", in: []string{"1", "2", "3"}, want: []int64{1, 2, 3}},
		{name: "bool", in: []bool{true, false, true}, want: []int64{1, 0, 1}},
		{name: "any", in: []any{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "bytes", in: [][]byte{{1}, {2}, {3}}, want: []int64{1, 2, 3}, wantFloat: []int64{0, 0, 0}},

		// Pointers to slices.
		{name: "int ptr", in: exSlicePtr([]int{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "int8 ptr", in: exSlicePtr([]int8{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "int16 ptr", in: exSlicePtr([]int16{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "int32 ptr", in: exSlicePtr([]int32{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "int64 ptr", in: exSlicePtr([]int64{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "uint ptr", in: exSlicePtr([]uint{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "uint8 ptr", in: exSlicePtr([]uint8{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "uint16 ptr", in: exSlicePtr([]uint16{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "uint32 ptr", in: exSlicePtr([]uint32{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "uint64 ptr", in: exSlicePtr([]uint64{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "float32 ptr", in: exSlicePtr([]float32{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "float64 ptr", in: exSlicePtr([]float64{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "complex64 ptr", in: exSlicePtr([]complex64{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "complex128 ptr", in: exSlicePtr([]complex128{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "string ptr", in: exSlicePtr([]string{"1", "2", "3"}), want: []int64{1, 2, 3}},
		{name: "bool ptr", in: exSlicePtr([]bool{true, false, true}), want: []int64{1, 0, 1}},
		{name: "any ptr", in: exSlicePtr([]any{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "bytes ptr", in: exSlicePtr([][]byte{{1}, {2}, {3}}), want: []int64{1, 2, 3}, wantFloat: []int64{0, 0, 0}},

		// Reflection fallbacks for named slice and array kinds.
		{name: "named slice", in: exInts{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "array", in: [3]int{1, 2, 3}, want: []int64{1, 2, 3}},

		// Nil pointers to slices, one per supported element type.
		{name: "nil int ptr", in: (*[]int)(nil), want: nil},
		{name: "nil int8 ptr", in: (*[]int8)(nil), want: nil},
		{name: "nil int16 ptr", in: (*[]int16)(nil), want: nil},
		{name: "nil int32 ptr", in: (*[]int32)(nil), want: nil},
		{name: "nil int64 ptr", in: (*[]int64)(nil), want: nil},
		{name: "nil uint ptr", in: (*[]uint)(nil), want: nil},
		{name: "nil uint8 ptr", in: (*[]uint8)(nil), want: nil},
		{name: "nil uint16 ptr", in: (*[]uint16)(nil), want: nil},
		{name: "nil uint32 ptr", in: (*[]uint32)(nil), want: nil},
		{name: "nil uint64 ptr", in: (*[]uint64)(nil), want: nil},
		{name: "nil float32 ptr", in: (*[]float32)(nil), want: nil},
		{name: "nil float64 ptr", in: (*[]float64)(nil), want: nil},
		{name: "nil complex64 ptr", in: (*[]complex64)(nil), want: nil},
		{name: "nil complex128 ptr", in: (*[]complex128)(nil), want: nil},
		{name: "nil string ptr", in: (*[]string)(nil), want: nil},
		{name: "nil bool ptr", in: (*[]bool)(nil), want: nil},
		{name: "nil any ptr", in: (*[]any)(nil), want: nil},
		{name: "nil bytes ptr", in: (*[][]byte)(nil), want: nil},
	}
}

// TestInts_Exhaustive verifies Ints for every supported slice element type.
func TestInts_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Ints", tt.in, xconv.Ints(tt.in), exTo[int](tt.want))
		})
	}
}

// TestInt8s_Exhaustive verifies Int8s for every supported slice element type.
func TestInt8s_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Int8s", tt.in, xconv.Int8s(tt.in), exTo[int8](tt.want))
		})
	}
}

// TestInt16s_Exhaustive verifies Int16s for every supported slice element type.
func TestInt16s_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Int16s", tt.in, xconv.Int16s(tt.in), exTo[int16](tt.want))
		})
	}
}

// TestInt32s_Exhaustive verifies Int32s for every supported slice element type.
func TestInt32s_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Int32s", tt.in, xconv.Int32s(tt.in), exTo[int32](tt.want))
		})
	}
}

// TestInt64s_Exhaustive verifies Int64s for every supported slice element type.
func TestInt64s_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Int64s", tt.in, xconv.Int64s(tt.in), exTo[int64](tt.want))
		})
	}
}

// TestUints_Exhaustive verifies Uints for every supported slice element type.
func TestUints_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Uints", tt.in, xconv.Uints(tt.in), exTo[uint](tt.want))
		})
	}
}

// TestUint8s_Exhaustive verifies Uint8s for every supported slice element type.
func TestUint8s_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Uint8s", tt.in, xconv.Uint8s(tt.in), exTo[uint8](tt.want))
		})
	}
}

// TestUint16s_Exhaustive verifies Uint16s for every supported slice element type.
func TestUint16s_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Uint16s", tt.in, xconv.Uint16s(tt.in), exTo[uint16](tt.want))
		})
	}
}

// TestUint32s_Exhaustive verifies Uint32s for every supported slice element type.
func TestUint32s_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Uint32s", tt.in, xconv.Uint32s(tt.in), exTo[uint32](tt.want))
		})
	}
}

// TestUint64s_Exhaustive verifies Uint64s for every supported slice element type.
func TestUint64s_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Uint64s", tt.in, xconv.Uint64s(tt.in), exTo[uint64](tt.want))
		})
	}
}

// TestFloat32s_Exhaustive verifies Float32s for every supported slice element type.
func TestFloat32s_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Float32s", tt.in, xconv.Float32s(tt.in), exTo[float32](tt.floatWant()))
		})
	}
}

// TestFloat64s_Exhaustive verifies Float64s for every supported slice element type.
func TestFloat64s_Exhaustive(t *testing.T) {
	for _, tt := range exSliceCorpus() {
		t.Run(tt.name, func(t *testing.T) {
			exAssertSlice(t, "Float64s", tt.in, xconv.Float64s(tt.in), exTo[float64](tt.floatWant()))
		})
	}
}
