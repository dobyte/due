package xconv_test

import (
	"testing"
	"time"
	"unsafe"

	"github.com/dobyte/due/v2/utils/xconv"
)

// numeric is the set of numeric kinds handled by the xconv numeric converters.
type numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// The named types below exercise the reflection fallback used by the converters
// for named basic and named slice types.
type (
	namedInt         int
	namedFloat       float64
	namedString      string
	namedIntSlice    []int
	namedStringSlice []string
)

// scalarCase describes a single scalar conversion case.
type scalarCase struct {
	name string
	in   any
	want int64
}

// sliceCase describes a single slice conversion case; wantNil means the result
// must be a nil slice, otherwise want holds the expected elements.
type sliceCase struct {
	name    string
	in      any
	want    []int64
	wantNil bool
}

// ptr returns a pointer to a copy of v so that pointer inputs can be built inline.
func ptr[T any](v T) *T {
	return &v
}

// scalarValueCases returns the shared table of scalar inputs whose converted
// value is non-zero for every numeric converter.
func scalarValueCases() []scalarCase {
	return []scalarCase{
		{name: "int", in: int(7), want: 7},
		{name: "int8", in: int8(7), want: 7},
		{name: "int16", in: int16(7), want: 7},
		{name: "int32", in: int32(7), want: 7},
		{name: "int64", in: int64(7), want: 7},
		{name: "uint", in: uint(7), want: 7},
		{name: "uint8", in: uint8(7), want: 7},
		{name: "uint16", in: uint16(7), want: 7},
		{name: "uint32", in: uint32(7), want: 7},
		{name: "uint64", in: uint64(7), want: 7},
		{name: "float32", in: float32(7), want: 7},
		{name: "float64", in: float64(7), want: 7},
		{name: "complex64", in: complex64(7 + 2i), want: 7},
		{name: "complex128", in: complex128(7 + 2i), want: 7},
		{name: "bool true", in: true, want: 1},
		{name: "string", in: "7", want: 7},
		{name: "named int", in: namedInt(7), want: 7},
		{name: "named float", in: namedFloat(7), want: 7},
		{name: "named string", in: namedString("7"), want: 7},
		{name: "uintptr", in: uintptr(7), want: 7},
		{name: "time", in: time.Unix(0, 7), want: 7},
		{name: "pointer to int", in: ptr(7), want: 7},
		{name: "pointer to float64", in: ptr(float64(7)), want: 7},
		{name: "pointer to complex128", in: ptr(complex128(7 + 2i)), want: 7},
		{name: "pointer to bool", in: ptr(true), want: 1},
		{name: "pointer to time", in: ptr(time.Unix(0, 7)), want: 7},
	}
}

// scalarZeroCases returns the shared table of scalar inputs whose converted
// value is zero.
func scalarZeroCases() []scalarCase {
	return []scalarCase{
		{name: "nil", in: nil, want: 0},
		{name: "nil int pointer", in: (*int)(nil), want: 0},
		{name: "nil int64 pointer", in: (*int64)(nil), want: 0},
		{name: "nil float64 pointer", in: (*float64)(nil), want: 0},
		{name: "nil bool pointer", in: (*bool)(nil), want: 0},
		{name: "nil complex pointer", in: (*complex128)(nil), want: 0},
		{name: "nil time pointer", in: (*time.Time)(nil), want: 0},
		{name: "empty string", in: "", want: 0},
		{name: "invalid string", in: "abc", want: 0},
		{name: "empty struct", in: struct{}{}, want: 0},
		{name: "oversized byte slice", in: make([]byte, 9), want: 0},
		{name: "func", in: func() {}, want: 0},
		{name: "bool false", in: false, want: 0},
		{name: "nil unsafe pointer", in: unsafe.Pointer(nil), want: 0},
	}
}

// scalarCases returns the value and zero scalar cases combined.
func scalarCases() []scalarCase {
	return append(scalarValueCases(), scalarZeroCases()...)
}

// integerScalarCases extends scalarCases with byte-slice inputs that only the
// integer converters turn into a non-zero value.
func integerScalarCases() []scalarCase {
	return append(scalarCases(), []scalarCase{
		{name: "byte slice", in: []byte{7}, want: 7},
		{name: "pointer to byte slice", in: ptr([]byte{7}), want: 7},
	}...)
}

// sliceValueCases returns the shared table of slice inputs whose converted
// elements are non-empty for every numeric converter.
func sliceValueCases() []sliceCase {
	return []sliceCase{
		{name: "int slice", in: []int{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "int slice pointer", in: ptr([]int{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "int8 slice", in: []int8{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "int16 slice", in: []int16{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "int32 slice", in: []int32{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "int64 slice", in: []int64{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "int64 slice pointer", in: ptr([]int64{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "uint slice", in: []uint{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "uint8 slice", in: []uint8{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "uint8 slice pointer", in: ptr([]uint8{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "uint16 slice", in: []uint16{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "uint32 slice", in: []uint32{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "uint64 slice", in: []uint64{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "float32 slice", in: []float32{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "float64 slice", in: []float64{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "float64 slice pointer", in: ptr([]float64{1, 2, 3}), want: []int64{1, 2, 3}},
		{name: "complex64 slice", in: []complex64{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "complex128 slice", in: []complex128{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "string slice", in: []string{"1", "2", "3"}, want: []int64{1, 2, 3}},
		{name: "string slice pointer", in: ptr([]string{"1", "2", "3"}), want: []int64{1, 2, 3}},
		{name: "bool slice", in: []bool{true, false, true}, want: []int64{1, 0, 1}},
		{name: "any slice", in: []any{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "int array", in: [3]int{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "named int slice", in: namedIntSlice{1, 2, 3}, want: []int64{1, 2, 3}},
		{name: "named string slice", in: namedStringSlice{"1", "2", "3"}, want: []int64{1, 2, 3}},
		{name: "empty int slice", in: []int{}, want: []int64{}},
	}
}

// sliceNilCases returns the shared table of slice inputs whose converted value
// is a nil slice.
func sliceNilCases() []sliceCase {
	return []sliceCase{
		{name: "nil", in: nil, wantNil: true},
		{name: "nil int slice pointer", in: (*[]int)(nil), wantNil: true},
		{name: "nil string slice pointer", in: (*[]string)(nil), wantNil: true},
		{name: "non-slice int", in: 7, wantNil: true},
		{name: "non-slice string", in: "123", wantNil: true},
		{name: "non-slice struct", in: struct{}{}, wantNil: true},
		{name: "non-slice func", in: func() {}, wantNil: true},
	}
}

// sliceCases returns the non-empty and nil slice cases combined.
func sliceCases() []sliceCase {
	return append(sliceValueCases(), sliceNilCases()...)
}

// integerSliceCases extends sliceCases with a byte-slice list that only the
// integer converters turn into non-zero elements.
func integerSliceCases() []sliceCase {
	return append(sliceCases(), sliceCase{
		name: "byte slice list",
		in:   [][]byte{{1}, {2}, {3}},
		want: []int64{1, 2, 3},
	})
}

// assertNumericSlice compares got against the expected elements converted to
// int64, or verifies that got is nil when wantNil is set.
func assertNumericSlice[T numeric](t *testing.T, got []T, want []int64, wantNil bool) {
	t.Helper()

	if wantNil {
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
		return
	}

	if got == nil {
		t.Errorf("got nil, want non-nil slice")
		return
	}

	if len(got) != len(want) {
		t.Errorf("len(got) = %d, want %d", len(got), len(want))
		return
	}

	for i := range got {
		if v := int64(got[i]); v != want[i] {
			t.Errorf("got[%d] = %d, want %d", i, v, want[i])
		}
	}
}

// TestInt_Table verifies Int conversions in a table-driven fashion.
func TestInt_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Int(tt.in), int(tt.want); got != want {
				t.Errorf("Int(%v) = %d, want %d", tt.in, got, want)
			}
		})
	}

	extra := []struct {
		name string
		in   any
		want int
	}{
		{name: "negative", in: -7, want: -7},
		{name: "float64 truncation", in: 7.9, want: 7},
		{name: "float32 truncation", in: float32(7.9), want: 7},
	}
	for _, tt := range extra {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Int(tt.in); got != tt.want {
				t.Errorf("Int(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestInt8_Table verifies Int8 conversions in a table-driven fashion.
func TestInt8_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Int8(tt.in), int8(tt.want); got != want {
				t.Errorf("Int8(%v) = %d, want %d", tt.in, got, want)
			}
		})
	}
}

// TestInt16_Table verifies Int16 conversions in a table-driven fashion.
func TestInt16_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Int16(tt.in), int16(tt.want); got != want {
				t.Errorf("Int16(%v) = %d, want %d", tt.in, got, want)
			}
		})
	}
}

// TestInt32_Table verifies Int32 conversions in a table-driven fashion.
func TestInt32_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Int32(tt.in), int32(tt.want); got != want {
				t.Errorf("Int32(%v) = %d, want %d", tt.in, got, want)
			}
		})
	}
}

// TestInt64_Table verifies Int64 conversions in a table-driven fashion.
func TestInt64_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Int64(tt.in), tt.want; got != want {
				t.Errorf("Int64(%v) = %d, want %d", tt.in, got, want)
			}
		})
	}

	extra := []struct {
		name string
		in   any
		want int64
	}{
		{name: "negative", in: -7, want: -7},
		{name: "float64 truncation", in: 7.9, want: 7},
	}
	for _, tt := range extra {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Int64(tt.in); got != tt.want {
				t.Errorf("Int64(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestUint_Table verifies Uint conversions in a table-driven fashion.
func TestUint_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Uint(tt.in), uint(tt.want); got != want {
				t.Errorf("Uint(%v) = %d, want %d", tt.in, got, want)
			}
		})
	}
}

// TestUint8_Table verifies Uint8 conversions in a table-driven fashion.
func TestUint8_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Uint8(tt.in), uint8(tt.want); got != want {
				t.Errorf("Uint8(%v) = %d, want %d", tt.in, got, want)
			}
		})
	}
}

// TestUint16_Table verifies Uint16 conversions in a table-driven fashion.
func TestUint16_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Uint16(tt.in), uint16(tt.want); got != want {
				t.Errorf("Uint16(%v) = %d, want %d", tt.in, got, want)
			}
		})
	}
}

// TestUint32_Table verifies Uint32 conversions in a table-driven fashion.
func TestUint32_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Uint32(tt.in), uint32(tt.want); got != want {
				t.Errorf("Uint32(%v) = %d, want %d", tt.in, got, want)
			}
		})
	}
}

// TestUint64_Table verifies Uint64 conversions in a table-driven fashion.
func TestUint64_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Uint64(tt.in), uint64(tt.want); got != want {
				t.Errorf("Uint64(%v) = %d, want %d", tt.in, got, want)
			}
		})
	}
}

// TestFloat32_Table verifies Float32 conversions in a table-driven fashion.
func TestFloat32_Table(t *testing.T) {
	for _, tt := range scalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Float32(tt.in), float32(tt.want); got != want {
				t.Errorf("Float32(%v) = %v, want %v", tt.in, got, want)
			}
		})
	}

	extra := []struct {
		name string
		in   any
		want float32
	}{
		{name: "fraction", in: 7.5, want: 7.5},
		{name: "string fraction", in: "7.5", want: 7.5},
	}
	for _, tt := range extra {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Float32(tt.in); got != tt.want {
				t.Errorf("Float32(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestFloat64_Table verifies Float64 conversions in a table-driven fashion.
func TestFloat64_Table(t *testing.T) {
	for _, tt := range scalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := xconv.Float64(tt.in), float64(tt.want); got != want {
				t.Errorf("Float64(%v) = %v, want %v", tt.in, got, want)
			}
		})
	}

	extra := []struct {
		name string
		in   any
		want float64
	}{
		{name: "fraction", in: 7.5, want: 7.5},
		{name: "float32 fraction", in: float32(7.5), want: 7.5},
		{name: "string fraction", in: "7.5", want: 7.5},
	}
	for _, tt := range extra {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Float64(tt.in); got != tt.want {
				t.Errorf("Float64(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestInts_Table verifies Ints conversions in a table-driven fashion.
func TestInts_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Ints(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestInt8s_Table verifies Int8s conversions in a table-driven fashion.
func TestInt8s_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Int8s(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestInt16s_Table verifies Int16s conversions in a table-driven fashion.
func TestInt16s_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Int16s(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestInt32s_Table verifies Int32s conversions in a table-driven fashion.
func TestInt32s_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Int32s(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestInt64s_Table verifies Int64s conversions in a table-driven fashion.
func TestInt64s_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Int64s(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestUints_Table verifies Uints conversions in a table-driven fashion.
func TestUints_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Uints(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestUint8s_Table verifies Uint8s conversions in a table-driven fashion.
func TestUint8s_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Uint8s(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestUint16s_Table verifies Uint16s conversions in a table-driven fashion.
func TestUint16s_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Uint16s(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestUint32s_Table verifies Uint32s conversions in a table-driven fashion.
func TestUint32s_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Uint32s(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestUint64s_Table verifies Uint64s conversions in a table-driven fashion.
func TestUint64s_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Uint64s(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestFloat32s_Table verifies Float32s conversions in a table-driven fashion.
func TestFloat32s_Table(t *testing.T) {
	for _, tt := range sliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Float32s(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestFloat64s_Table verifies Float64s conversions in a table-driven fashion.
func TestFloat64s_Table(t *testing.T) {
	for _, tt := range sliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			assertNumericSlice(t, xconv.Float64s(tt.in), tt.want, tt.wantNil)
		})
	}
}

// TestIntPointer_Table verifies IntPointer conversions in a table-driven fashion.
func TestIntPointer_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.IntPointer(tt.in)
			if got == nil {
				t.Fatalf("IntPointer(%v) = nil, want non-nil", tt.in)
			}
			if want := int(tt.want); *got != want {
				t.Errorf("IntPointer(%v) = %d, want %d", tt.in, *got, want)
			}
		})
	}
}

// TestInt8Pointer_Table verifies Int8Pointer conversions in a table-driven fashion.
func TestInt8Pointer_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Int8Pointer(tt.in)
			if got == nil {
				t.Fatalf("Int8Pointer(%v) = nil, want non-nil", tt.in)
			}
			if want := int8(tt.want); *got != want {
				t.Errorf("Int8Pointer(%v) = %d, want %d", tt.in, *got, want)
			}
		})
	}
}

// TestInt16Pointer_Table verifies Int16Pointer conversions in a table-driven fashion.
func TestInt16Pointer_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Int16Pointer(tt.in)
			if got == nil {
				t.Fatalf("Int16Pointer(%v) = nil, want non-nil", tt.in)
			}
			if want := int16(tt.want); *got != want {
				t.Errorf("Int16Pointer(%v) = %d, want %d", tt.in, *got, want)
			}
		})
	}
}

// TestInt32Pointer_Table verifies Int32Pointer conversions in a table-driven fashion.
func TestInt32Pointer_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Int32Pointer(tt.in)
			if got == nil {
				t.Fatalf("Int32Pointer(%v) = nil, want non-nil", tt.in)
			}
			if want := int32(tt.want); *got != want {
				t.Errorf("Int32Pointer(%v) = %d, want %d", tt.in, *got, want)
			}
		})
	}
}

// TestInt64Pointer_Table verifies Int64Pointer conversions in a table-driven fashion.
func TestInt64Pointer_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Int64Pointer(tt.in)
			if got == nil {
				t.Fatalf("Int64Pointer(%v) = nil, want non-nil", tt.in)
			}
			if *got != tt.want {
				t.Errorf("Int64Pointer(%v) = %d, want %d", tt.in, *got, tt.want)
			}
		})
	}
}

// TestUintPointer_Table verifies UintPointer conversions in a table-driven fashion.
func TestUintPointer_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.UintPointer(tt.in)
			if got == nil {
				t.Fatalf("UintPointer(%v) = nil, want non-nil", tt.in)
			}
			if want := uint(tt.want); *got != want {
				t.Errorf("UintPointer(%v) = %d, want %d", tt.in, *got, want)
			}
		})
	}
}

// TestUint8Pointer_Table verifies Uint8Pointer conversions in a table-driven fashion.
func TestUint8Pointer_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Uint8Pointer(tt.in)
			if got == nil {
				t.Fatalf("Uint8Pointer(%v) = nil, want non-nil", tt.in)
			}
			if want := uint8(tt.want); *got != want {
				t.Errorf("Uint8Pointer(%v) = %d, want %d", tt.in, *got, want)
			}
		})
	}
}

// TestUint16Pointer_Table verifies Uint16Pointer conversions in a table-driven fashion.
func TestUint16Pointer_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Uint16Pointer(tt.in)
			if got == nil {
				t.Fatalf("Uint16Pointer(%v) = nil, want non-nil", tt.in)
			}
			if want := uint16(tt.want); *got != want {
				t.Errorf("Uint16Pointer(%v) = %d, want %d", tt.in, *got, want)
			}
		})
	}
}

// TestUint32Pointer_Table verifies Uint32Pointer conversions in a table-driven fashion.
func TestUint32Pointer_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Uint32Pointer(tt.in)
			if got == nil {
				t.Fatalf("Uint32Pointer(%v) = nil, want non-nil", tt.in)
			}
			if want := uint32(tt.want); *got != want {
				t.Errorf("Uint32Pointer(%v) = %d, want %d", tt.in, *got, want)
			}
		})
	}
}

// TestUint64Pointer_Table verifies Uint64Pointer conversions in a table-driven fashion.
func TestUint64Pointer_Table(t *testing.T) {
	for _, tt := range integerScalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Uint64Pointer(tt.in)
			if got == nil {
				t.Fatalf("Uint64Pointer(%v) = nil, want non-nil", tt.in)
			}
			if want := uint64(tt.want); *got != want {
				t.Errorf("Uint64Pointer(%v) = %d, want %d", tt.in, *got, want)
			}
		})
	}
}

// TestFloat32Pointer_Table verifies Float32Pointer conversions in a table-driven fashion.
func TestFloat32Pointer_Table(t *testing.T) {
	for _, tt := range scalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Float32Pointer(tt.in)
			if got == nil {
				t.Fatalf("Float32Pointer(%v) = nil, want non-nil", tt.in)
			}
			if want := float32(tt.want); *got != want {
				t.Errorf("Float32Pointer(%v) = %v, want %v", tt.in, *got, want)
			}
		})
	}
}

// TestFloat64Pointer_Table verifies Float64Pointer conversions in a table-driven fashion.
func TestFloat64Pointer_Table(t *testing.T) {
	for _, tt := range scalarCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Float64Pointer(tt.in)
			if got == nil {
				t.Fatalf("Float64Pointer(%v) = nil, want non-nil", tt.in)
			}
			if want := float64(tt.want); *got != want {
				t.Errorf("Float64Pointer(%v) = %v, want %v", tt.in, *got, want)
			}
		})
	}
}

// TestIntsPointer_Table verifies IntsPointer conversions in a table-driven fashion.
func TestIntsPointer_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.IntsPointer(tt.in)
			if got == nil {
				t.Fatalf("IntsPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}

// TestInt8sPointer_Table verifies Int8sPointer conversions in a table-driven fashion.
func TestInt8sPointer_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Int8sPointer(tt.in)
			if got == nil {
				t.Fatalf("Int8sPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}

// TestInt16sPointer_Table verifies Int16sPointer conversions in a table-driven fashion.
func TestInt16sPointer_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Int16sPointer(tt.in)
			if got == nil {
				t.Fatalf("Int16sPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}

// TestInt32sPointer_Table verifies Int32sPointer conversions in a table-driven fashion.
func TestInt32sPointer_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Int32sPointer(tt.in)
			if got == nil {
				t.Fatalf("Int32sPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}

// TestInt64sPointer_Table verifies Int64sPointer conversions in a table-driven fashion.
func TestInt64sPointer_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Int64sPointer(tt.in)
			if got == nil {
				t.Fatalf("Int64sPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}

// TestUintsPointer_Table verifies UintsPointer conversions in a table-driven fashion.
func TestUintsPointer_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.UintsPointer(tt.in)
			if got == nil {
				t.Fatalf("UintsPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}

// TestUint8sPointer_Table verifies Uint8sPointer conversions in a table-driven fashion.
func TestUint8sPointer_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Uint8sPointer(tt.in)
			if got == nil {
				t.Fatalf("Uint8sPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}

// TestUint16sPointer_Table verifies Uint16sPointer conversions in a table-driven fashion.
func TestUint16sPointer_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Uint16sPointer(tt.in)
			if got == nil {
				t.Fatalf("Uint16sPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}

// TestUint32sPointer_Table verifies Uint32sPointer conversions in a table-driven fashion.
func TestUint32sPointer_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Uint32sPointer(tt.in)
			if got == nil {
				t.Fatalf("Uint32sPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}

// TestUint64sPointer_Table verifies Uint64sPointer conversions in a table-driven fashion.
func TestUint64sPointer_Table(t *testing.T) {
	for _, tt := range integerSliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Uint64sPointer(tt.in)
			if got == nil {
				t.Fatalf("Uint64sPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}

// TestFloat32sPointer_Table verifies Float32sPointer conversions in a table-driven fashion.
func TestFloat32sPointer_Table(t *testing.T) {
	for _, tt := range sliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Float32sPointer(tt.in)
			if got == nil {
				t.Fatalf("Float32sPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}

// TestFloat64sPointer_Table verifies Float64sPointer conversions in a table-driven fashion.
func TestFloat64sPointer_Table(t *testing.T) {
	for _, tt := range sliceCases() {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.Float64sPointer(tt.in)
			if got == nil {
				t.Fatalf("Float64sPointer(%v) = nil, want non-nil", tt.in)
			}
			assertNumericSlice(t, *got, tt.want, tt.wantNil)
		})
	}
}
