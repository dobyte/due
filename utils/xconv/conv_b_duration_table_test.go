package xconv_test

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/dobyte/due/v2/utils/xconv"
)

// The named types below exercise the reflection fallback branches of B, Bs,
// Duration and Durations for named basic and named slice types.
type (
	bdInt     int
	bdUint    uint
	bdFloat64 float64
	bdComplex complex128
	bdString  string
	bdBool    bool
	bdInts    []int
)

// TestB_Exhaustive verifies B for every supported scalar type, its pointer form,
// the named/reflection fallbacks and unsupported inputs.
func TestB_Exhaustive(t *testing.T) {
	var bdUnsafeTarget int
	bdUnsafePtr := unsafe.Pointer(&bdUnsafeTarget)

	cases := []struct {
		name string
		in   any
		want float64
	}{
		{name: "nil", in: nil, want: 0},
		{name: "int", in: 7, want: 7},
		{name: "int pointer", in: ptrTo(7), want: 7},
		{name: "nil int pointer", in: (*int)(nil), want: 0},
		{name: "int8", in: int8(-8), want: -8},
		{name: "int8 pointer", in: ptrTo(int8(-8)), want: -8},
		{name: "nil int8 pointer", in: (*int8)(nil), want: 0},
		{name: "int16", in: int16(16), want: 16},
		{name: "int16 pointer", in: ptrTo(int16(16)), want: 16},
		{name: "nil int16 pointer", in: (*int16)(nil), want: 0},
		{name: "int32", in: int32(32), want: 32},
		{name: "int32 pointer", in: ptrTo(int32(32)), want: 32},
		{name: "nil int32 pointer", in: (*int32)(nil), want: 0},
		{name: "int64", in: int64(64), want: 64},
		{name: "int64 pointer", in: ptrTo(int64(64)), want: 64},
		{name: "nil int64 pointer", in: (*int64)(nil), want: 0},
		{name: "uint", in: uint(5), want: 5},
		{name: "uint pointer", in: ptrTo(uint(5)), want: 5},
		{name: "nil uint pointer", in: (*uint)(nil), want: 0},
		{name: "uint8", in: uint8(8), want: 8},
		{name: "uint8 pointer", in: ptrTo(uint8(8)), want: 8},
		{name: "nil uint8 pointer", in: (*uint8)(nil), want: 0},
		{name: "uint16", in: uint16(16), want: 16},
		{name: "uint16 pointer", in: ptrTo(uint16(16)), want: 16},
		{name: "nil uint16 pointer", in: (*uint16)(nil), want: 0},
		{name: "uint32", in: uint32(32), want: 32},
		{name: "uint32 pointer", in: ptrTo(uint32(32)), want: 32},
		{name: "nil uint32 pointer", in: (*uint32)(nil), want: 0},
		{name: "uint64", in: uint64(64), want: 64},
		{name: "uint64 pointer", in: ptrTo(uint64(64)), want: 64},
		{name: "nil uint64 pointer", in: (*uint64)(nil), want: 0},
		{name: "float32", in: float32(1.5), want: 1.5},
		{name: "float32 pointer", in: ptrTo(float32(1.5)), want: 1.5},
		{name: "nil float32 pointer", in: (*float32)(nil), want: 0},
		{name: "float64", in: 2.5, want: 2.5},
		{name: "float64 pointer", in: ptrTo(2.5), want: 2.5},
		{name: "nil float64 pointer", in: (*float64)(nil), want: 0},
		{name: "complex64", in: complex64(complex(3, 4)), want: 3},
		{name: "complex64 pointer", in: ptrTo(complex64(complex(3, 4))), want: 3},
		{name: "nil complex64 pointer", in: (*complex64)(nil), want: 0},
		{name: "complex128", in: complex128(complex(5, 6)), want: 5},
		{name: "complex128 pointer", in: ptrTo(complex128(complex(5, 6))), want: 5},
		{name: "nil complex128 pointer", in: (*complex128)(nil), want: 0},
		{name: "bool", in: true, want: 0},
		{name: "bool false", in: false, want: 0},
		{name: "bool pointer", in: ptrTo(true), want: 0},
		{name: "string", in: "1KB", want: float64(1 << 10)},
		{name: "string bare number", in: "123", want: 123},
		{name: "string pointer", in: ptrTo("1KB"), want: float64(1 << 10)},
		{name: "nil string pointer", in: (*string)(nil), want: 0},
		{name: "bytes", in: []byte("1KB"), want: float64(1 << 10)},
		{name: "bytes pointer", in: ptrTo([]byte("1KB")), want: float64(1 << 10)},
		{name: "nil bytes pointer", in: (*[]byte)(nil), want: 0},
		{name: "time", in: time.Time{}, want: 0},
		{name: "time pointer", in: ptrTo(time.Time{}), want: 0},
		{name: "nil time pointer", in: (*time.Time)(nil), want: 0},
		{name: "duration", in: time.Duration(5), want: 0},
		{name: "duration pointer", in: ptrTo(time.Duration(5)), want: 0},
		{name: "nil duration pointer", in: (*time.Duration)(nil), want: 0},
		{name: "named string", in: bdString("2KB"), want: float64(2 << 10)},
		{name: "uintptr", in: uintptr(7), want: 7},
		{name: "unsafe pointer", in: bdUnsafePtr, want: float64(uintptr(bdUnsafePtr))},
		{name: "named int", in: bdInt(9), want: 9},
		{name: "named uint", in: bdUint(11), want: 11},
		{name: "named float", in: bdFloat64(1.5), want: 1.5},
		{name: "named complex", in: bdComplex(complex(2, 3)), want: 2},
		{name: "nil unsafe pointer", in: unsafe.Pointer(nil), want: 0},
		{name: "struct", in: struct{}{}, want: 0},
		{name: "map", in: map[string]int{"a": 1}, want: 0},
		{name: "chan", in: make(chan int), want: 0},
		{name: "func", in: func() {}, want: 0},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.B(tt.in); got != tt.want {
				t.Errorf("B(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestBs_Exhaustive verifies Bs for every supported slice element type, its
// pointer form, the reflection fallback and unsupported inputs.
func TestBs_Exhaustive(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []float64
	}{
		{name: "nil", in: nil, want: nil},
		{name: "int slice", in: []int{1, 2}, want: []float64{1, 2}},
		{name: "int slice pointer", in: ptrTo([]int{1, 2}), want: []float64{1, 2}},
		{name: "nil int slice pointer", in: (*[]int)(nil), want: nil},
		{name: "int8 slice", in: []int8{-1, 2}, want: []float64{-1, 2}},
		{name: "int8 slice pointer", in: ptrTo([]int8{-1, 2}), want: []float64{-1, 2}},
		{name: "nil int8 slice pointer", in: (*[]int8)(nil), want: nil},
		{name: "int16 slice", in: []int16{1, 2}, want: []float64{1, 2}},
		{name: "int16 slice pointer", in: ptrTo([]int16{1, 2}), want: []float64{1, 2}},
		{name: "nil int16 slice pointer", in: (*[]int16)(nil), want: nil},
		{name: "int32 slice", in: []int32{1, 2}, want: []float64{1, 2}},
		{name: "int32 slice pointer", in: ptrTo([]int32{1, 2}), want: []float64{1, 2}},
		{name: "nil int32 slice pointer", in: (*[]int32)(nil), want: nil},
		{name: "int64 slice", in: []int64{1, 2}, want: []float64{1, 2}},
		{name: "int64 slice pointer", in: ptrTo([]int64{1, 2}), want: []float64{1, 2}},
		{name: "nil int64 slice pointer", in: (*[]int64)(nil), want: nil},
		{name: "uint slice", in: []uint{1, 2}, want: []float64{1, 2}},
		{name: "uint slice pointer", in: ptrTo([]uint{1, 2}), want: []float64{1, 2}},
		{name: "nil uint slice pointer", in: (*[]uint)(nil), want: nil},
		{name: "uint8 slice", in: []uint8{1, 2}, want: []float64{1, 2}},
		{name: "uint8 slice pointer", in: ptrTo([]uint8{1, 2}), want: []float64{1, 2}},
		{name: "nil uint8 slice pointer", in: (*[]uint8)(nil), want: nil},
		{name: "uint16 slice", in: []uint16{1, 2}, want: []float64{1, 2}},
		{name: "uint16 slice pointer", in: ptrTo([]uint16{1, 2}), want: []float64{1, 2}},
		{name: "nil uint16 slice pointer", in: (*[]uint16)(nil), want: nil},
		{name: "uint32 slice", in: []uint32{1, 2}, want: []float64{1, 2}},
		{name: "uint32 slice pointer", in: ptrTo([]uint32{1, 2}), want: []float64{1, 2}},
		{name: "nil uint32 slice pointer", in: (*[]uint32)(nil), want: nil},
		{name: "uint64 slice", in: []uint64{1, 2}, want: []float64{1, 2}},
		{name: "uint64 slice pointer", in: ptrTo([]uint64{1, 2}), want: []float64{1, 2}},
		{name: "nil uint64 slice pointer", in: (*[]uint64)(nil), want: nil},
		{name: "float32 slice", in: []float32{1.5, 2.5}, want: []float64{1.5, 2.5}},
		{name: "float32 slice pointer", in: ptrTo([]float32{1.5, 2.5}), want: []float64{1.5, 2.5}},
		{name: "nil float32 slice pointer", in: (*[]float32)(nil), want: nil},
		{name: "float64 slice", in: []float64{1.5, 2.5}, want: []float64{1.5, 2.5}},
		{name: "float64 slice pointer", in: ptrTo([]float64{1.5, 2.5}), want: []float64{1.5, 2.5}},
		{name: "nil float64 slice pointer", in: (*[]float64)(nil), want: nil},
		{name: "complex64 slice", in: []complex64{complex(1, 1), complex(2, 2)}, want: []float64{1, 2}},
		{name: "complex64 slice pointer", in: ptrTo([]complex64{complex(1, 1), complex(2, 2)}), want: []float64{1, 2}},
		{name: "nil complex64 slice pointer", in: (*[]complex64)(nil), want: nil},
		{name: "complex128 slice", in: []complex128{complex(3, 1), complex(4, 2)}, want: []float64{3, 4}},
		{name: "complex128 slice pointer", in: ptrTo([]complex128{complex(3, 1), complex(4, 2)}), want: []float64{3, 4}},
		{name: "nil complex128 slice pointer", in: (*[]complex128)(nil), want: nil},
		{name: "string slice", in: []string{"1KB", "1MB"}, want: []float64{float64(1 << 10), float64(1 << 20)}},
		{name: "string slice pointer", in: ptrTo([]string{"1KB"}), want: []float64{float64(1 << 10)}},
		{name: "nil string slice pointer", in: (*[]string)(nil), want: nil},
		{name: "bool slice", in: []bool{true, false}, want: []float64{0, 0}},
		{name: "bool slice pointer", in: ptrTo([]bool{true, false}), want: []float64{0, 0}},
		{name: "nil bool slice pointer", in: (*[]bool)(nil), want: nil},
		{name: "any slice", in: []any{1, "1KB", true, nil}, want: []float64{1, float64(1 << 10), 0, 0}},
		{name: "any slice pointer", in: ptrTo([]any{1, "1KB"}), want: []float64{1, float64(1 << 10)}},
		{name: "nil any slice pointer", in: (*[]any)(nil), want: nil},
		{name: "bytes slice", in: [][]byte{[]byte("1KB"), []byte("1MB")}, want: []float64{float64(1 << 10), float64(1 << 20)}},
		{name: "bytes slice pointer", in: ptrTo([][]byte{[]byte("1KB")}), want: []float64{float64(1 << 10)}},
		{name: "nil bytes slice pointer", in: (*[][]byte)(nil), want: nil},
		{name: "named int slice", in: bdInts{3, 4}, want: []float64{3, 4}},
		{name: "array", in: [3]int{1, 2, 3}, want: []float64{1, 2, 3}},
		{name: "unsupported struct", in: struct{}{}, want: nil},
		{name: "unsupported map", in: map[string]int{"a": 1}, want: nil},
		{name: "unsupported chan", in: make(chan int), want: nil},
		{name: "unsupported func", in: func() {}, want: nil},
		{name: "unsupported scalar", in: 123, want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Bs(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bs(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestDuration_Exhaustive verifies Duration for every supported scalar type, its
// pointer form, unit strings, the named/reflection fallbacks and unsupported inputs.
func TestDuration_Exhaustive(t *testing.T) {
	var bdUnsafeTarget int
	bdUnsafePtr := unsafe.Pointer(&bdUnsafeTarget)

	bdLongDay := "999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999d"

	cases := []struct {
		name string
		in   any
		want time.Duration
	}{
		{name: "nil", in: nil, want: 0},
		{name: "int", in: 7, want: time.Duration(7)},
		{name: "int pointer", in: ptrTo(7), want: time.Duration(7)},
		{name: "nil int pointer", in: (*int)(nil), want: 0},
		{name: "int8", in: int8(-8), want: time.Duration(-8)},
		{name: "int8 pointer", in: ptrTo(int8(-8)), want: time.Duration(-8)},
		{name: "nil int8 pointer", in: (*int8)(nil), want: 0},
		{name: "int16", in: int16(16), want: time.Duration(16)},
		{name: "int16 pointer", in: ptrTo(int16(16)), want: time.Duration(16)},
		{name: "nil int16 pointer", in: (*int16)(nil), want: 0},
		{name: "int32", in: int32(32), want: time.Duration(32)},
		{name: "int32 pointer", in: ptrTo(int32(32)), want: time.Duration(32)},
		{name: "nil int32 pointer", in: (*int32)(nil), want: 0},
		{name: "int64", in: int64(64), want: time.Duration(64)},
		{name: "int64 pointer", in: ptrTo(int64(64)), want: time.Duration(64)},
		{name: "nil int64 pointer", in: (*int64)(nil), want: 0},
		{name: "uint", in: uint(5), want: time.Duration(5)},
		{name: "uint pointer", in: ptrTo(uint(5)), want: time.Duration(5)},
		{name: "nil uint pointer", in: (*uint)(nil), want: 0},
		{name: "uint8", in: uint8(8), want: time.Duration(8)},
		{name: "uint8 pointer", in: ptrTo(uint8(8)), want: time.Duration(8)},
		{name: "nil uint8 pointer", in: (*uint8)(nil), want: 0},
		{name: "uint16", in: uint16(16), want: time.Duration(16)},
		{name: "uint16 pointer", in: ptrTo(uint16(16)), want: time.Duration(16)},
		{name: "nil uint16 pointer", in: (*uint16)(nil), want: 0},
		{name: "uint32", in: uint32(32), want: time.Duration(32)},
		{name: "uint32 pointer", in: ptrTo(uint32(32)), want: time.Duration(32)},
		{name: "nil uint32 pointer", in: (*uint32)(nil), want: 0},
		{name: "uint64", in: uint64(64), want: time.Duration(64)},
		{name: "uint64 pointer", in: ptrTo(uint64(64)), want: time.Duration(64)},
		{name: "nil uint64 pointer", in: (*uint64)(nil), want: 0},
		{name: "float32", in: float32(1.9), want: time.Duration(1)},
		{name: "float32 pointer", in: ptrTo(float32(1.9)), want: time.Duration(1)},
		{name: "nil float32 pointer", in: (*float32)(nil), want: 0},
		{name: "float64", in: 2.9, want: time.Duration(2)},
		{name: "float64 pointer", in: ptrTo(2.9), want: time.Duration(2)},
		{name: "nil float64 pointer", in: (*float64)(nil), want: 0},
		{name: "complex64", in: complex64(complex(3, 4)), want: time.Duration(3)},
		{name: "complex64 pointer", in: ptrTo(complex64(complex(3, 4))), want: time.Duration(3)},
		{name: "nil complex64 pointer", in: (*complex64)(nil), want: 0},
		{name: "complex128", in: complex128(complex(5, 6)), want: time.Duration(5)},
		{name: "complex128 pointer", in: ptrTo(complex128(complex(5, 6))), want: time.Duration(5)},
		{name: "nil complex128 pointer", in: (*complex128)(nil), want: 0},
		{name: "bool", in: true, want: 0},
		{name: "bool false", in: false, want: 0},
		{name: "bool pointer", in: ptrTo(true), want: 0},
		{name: "nil bool pointer", in: (*bool)(nil), want: 0},
		{name: "seconds", in: "1s", want: time.Second},
		{name: "milliseconds", in: "1ms", want: time.Millisecond},
		{name: "microseconds", in: "1us", want: time.Microsecond},
		{name: "micro sign", in: "1µs", want: time.Microsecond},
		{name: "nanoseconds", in: "1ns", want: time.Nanosecond},
		{name: "minutes", in: "1m", want: time.Minute},
		{name: "hours", in: "1h", want: time.Hour},
		{name: "one day", in: "1d", want: 24 * time.Hour},
		{name: "one and a half day", in: "1.5d", want: 36 * time.Hour},
		{name: "day and hour", in: "1d2h", want: 26 * time.Hour},
		{name: "negative day", in: "-1d", want: -24 * time.Hour},
		{name: "uppercase", in: "1S", want: time.Second},
		{name: "zero seconds", in: "0s", want: 0},
		{name: "invalid string", in: "abc", want: 0},
		{name: "bare number", in: "1000", want: 0},
		{name: "overflow day", in: bdLongDay, want: 0},
		{name: "string pointer", in: ptrTo("1s"), want: time.Second},
		{name: "nil string pointer", in: (*string)(nil), want: 0},
		{name: "bytes", in: []byte("2s"), want: 2 * time.Second},
		{name: "bytes pointer", in: ptrTo([]byte("2s")), want: 2 * time.Second},
		{name: "nil bytes pointer", in: (*[]byte)(nil), want: 0},
		{name: "time", in: time.Unix(1, 2), want: time.Duration(1000000002)},
		{name: "time pointer", in: ptrTo(time.Unix(1, 2)), want: time.Duration(1000000002)},
		{name: "nil time pointer", in: (*time.Time)(nil), want: 0},
		{name: "duration", in: time.Duration(3), want: time.Duration(3)},
		{name: "duration pointer", in: ptrTo(time.Duration(4)), want: time.Duration(4)},
		{name: "nil duration pointer", in: (*time.Duration)(nil), want: 0},
		{name: "named bool", in: bdBool(true), want: 0},
		{name: "named string", in: bdString("1s"), want: time.Second},
		{name: "uintptr", in: uintptr(7), want: time.Duration(7)},
		{name: "unsafe pointer", in: bdUnsafePtr, want: time.Duration(uintptr(bdUnsafePtr))},
		{name: "named int", in: bdInt(9), want: time.Duration(9)},
		{name: "named uint", in: bdUint(11), want: time.Duration(11)},
		{name: "named float", in: bdFloat64(12.9), want: time.Duration(12)},
		{name: "named complex", in: bdComplex(complex(13, 1)), want: time.Duration(13)},
		{name: "struct", in: struct{}{}, want: 0},
		{name: "map", in: map[string]int{"a": 1}, want: 0},
		{name: "chan", in: make(chan int), want: 0},
		{name: "func", in: func() {}, want: 0},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Duration(tt.in); got != tt.want {
				t.Errorf("Duration(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestDurations_Exhaustive verifies Durations for every supported slice element
// type, its pointer form, the reflection fallback and unsupported inputs.
func TestDurations_Exhaustive(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []time.Duration
	}{
		{name: "nil", in: nil, want: nil},
		{name: "int slice", in: []int{1, 2}, want: []time.Duration{1, 2}},
		{name: "int slice pointer", in: ptrTo([]int{1, 2}), want: []time.Duration{1, 2}},
		{name: "nil int slice pointer", in: (*[]int)(nil), want: nil},
		{name: "int8 slice", in: []int8{-1, 2}, want: []time.Duration{-1, 2}},
		{name: "int8 slice pointer", in: ptrTo([]int8{-1, 2}), want: []time.Duration{-1, 2}},
		{name: "nil int8 slice pointer", in: (*[]int8)(nil), want: nil},
		{name: "int16 slice", in: []int16{1, 2}, want: []time.Duration{1, 2}},
		{name: "int16 slice pointer", in: ptrTo([]int16{1, 2}), want: []time.Duration{1, 2}},
		{name: "nil int16 slice pointer", in: (*[]int16)(nil), want: nil},
		{name: "int32 slice", in: []int32{1, 2}, want: []time.Duration{1, 2}},
		{name: "int32 slice pointer", in: ptrTo([]int32{1, 2}), want: []time.Duration{1, 2}},
		{name: "nil int32 slice pointer", in: (*[]int32)(nil), want: nil},
		{name: "int64 slice", in: []int64{1, 2}, want: []time.Duration{1, 2}},
		{name: "int64 slice pointer", in: ptrTo([]int64{1, 2}), want: []time.Duration{1, 2}},
		{name: "nil int64 slice pointer", in: (*[]int64)(nil), want: nil},
		{name: "uint slice", in: []uint{1, 2}, want: []time.Duration{1, 2}},
		{name: "uint slice pointer", in: ptrTo([]uint{1, 2}), want: []time.Duration{1, 2}},
		{name: "nil uint slice pointer", in: (*[]uint)(nil), want: nil},
		{name: "uint8 slice", in: []uint8{1, 2}, want: []time.Duration{1, 2}},
		{name: "uint8 slice pointer", in: ptrTo([]uint8{1, 2}), want: []time.Duration{1, 2}},
		{name: "nil uint8 slice pointer", in: (*[]uint8)(nil), want: nil},
		{name: "uint16 slice", in: []uint16{1, 2}, want: []time.Duration{1, 2}},
		{name: "uint16 slice pointer", in: ptrTo([]uint16{1, 2}), want: []time.Duration{1, 2}},
		{name: "nil uint16 slice pointer", in: (*[]uint16)(nil), want: nil},
		{name: "uint32 slice", in: []uint32{1, 2}, want: []time.Duration{1, 2}},
		{name: "uint32 slice pointer", in: ptrTo([]uint32{1, 2}), want: []time.Duration{1, 2}},
		{name: "nil uint32 slice pointer", in: (*[]uint32)(nil), want: nil},
		{name: "uint64 slice", in: []uint64{1, 2}, want: []time.Duration{1, 2}},
		{name: "uint64 slice pointer", in: ptrTo([]uint64{1, 2}), want: []time.Duration{1, 2}},
		{name: "nil uint64 slice pointer", in: (*[]uint64)(nil), want: nil},
		{name: "float32 slice", in: []float32{1.9, 2.9}, want: []time.Duration{1, 2}},
		{name: "float32 slice pointer", in: ptrTo([]float32{1.9, 2.9}), want: []time.Duration{1, 2}},
		{name: "nil float32 slice pointer", in: (*[]float32)(nil), want: nil},
		{name: "float64 slice", in: []float64{1.9, 2.9}, want: []time.Duration{1, 2}},
		{name: "float64 slice pointer", in: ptrTo([]float64{1.9, 2.9}), want: []time.Duration{1, 2}},
		{name: "nil float64 slice pointer", in: (*[]float64)(nil), want: nil},
		{name: "complex64 slice", in: []complex64{complex(1, 1), complex(2, 2)}, want: []time.Duration{1, 2}},
		{name: "complex64 slice pointer", in: ptrTo([]complex64{complex(1, 1), complex(2, 2)}), want: []time.Duration{1, 2}},
		{name: "nil complex64 slice pointer", in: (*[]complex64)(nil), want: nil},
		{name: "complex128 slice", in: []complex128{complex(3, 1), complex(4, 2)}, want: []time.Duration{3, 4}},
		{name: "complex128 slice pointer", in: ptrTo([]complex128{complex(3, 1), complex(4, 2)}), want: []time.Duration{3, 4}},
		{name: "nil complex128 slice pointer", in: (*[]complex128)(nil), want: nil},
		{name: "string slice", in: []string{"1s", "1m"}, want: []time.Duration{time.Second, time.Minute}},
		{name: "string slice pointer", in: ptrTo([]string{"1s"}), want: []time.Duration{time.Second}},
		{name: "nil string slice pointer", in: (*[]string)(nil), want: nil},
		{name: "bool slice", in: []bool{true, false}, want: []time.Duration{0, 0}},
		{name: "bool slice pointer", in: ptrTo([]bool{true, false}), want: []time.Duration{0, 0}},
		{name: "nil bool slice pointer", in: (*[]bool)(nil), want: nil},
		{name: "any slice", in: []any{"1s", 1000, true, nil}, want: []time.Duration{time.Second, 1000, 0, 0}},
		{name: "any slice pointer", in: ptrTo([]any{"1s", 1000}), want: []time.Duration{time.Second, 1000}},
		{name: "nil any slice pointer", in: (*[]any)(nil), want: nil},
		{name: "bytes slice", in: [][]byte{[]byte("1s"), []byte("2s")}, want: []time.Duration{time.Second, 2 * time.Second}},
		{name: "bytes slice pointer", in: ptrTo([][]byte{[]byte("1s")}), want: []time.Duration{time.Second}},
		{name: "nil bytes slice pointer", in: (*[][]byte)(nil), want: nil},
		{name: "named int slice", in: bdInts{3, 4}, want: []time.Duration{3, 4}},
		{name: "array", in: [3]int{1, 2, 3}, want: []time.Duration{1, 2, 3}},
		{name: "unsupported struct", in: struct{}{}, want: nil},
		{name: "unsupported map", in: map[string]int{"a": 1}, want: nil},
		{name: "unsupported chan", in: make(chan int), want: nil},
		{name: "unsupported func", in: func() {}, want: nil},
		{name: "unsupported scalar", in: 123, want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Durations(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Durations(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
