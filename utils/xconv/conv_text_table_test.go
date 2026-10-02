package xconv_test

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/dobyte/due/v2/utils/xconv"
)

// The named types below exercise the reflection fallback of Bool, String and Bytes.
type (
	txInt        int
	txInt8       int8
	txInt16      int16
	txInt32      int32
	txInt64      int64
	txUint       uint
	txUint8      uint8
	txUint16     uint16
	txUint32     uint32
	txUint64     uint64
	txFloat32    float32
	txFloat64    float64
	txComplex64  complex64
	txComplex128 complex128
	txBool       bool
	txStr        string
	txIntSlice   []int
	txStrSlice   []string
	txBoolSlice  []bool
)

// txChanHolder marshals to JSON unsuccessfully because of its channel field.
type txChanHolder struct {
	C chan int
}

// txPtr returns a pointer to a copy of v so that pointer inputs can be built inline.
func txPtr[T any](v T) *T {
	return &v
}

// TestBool_Exhaustive feeds every supported input type into Bool.
func TestBool_Exhaustive(t *testing.T) {
	n := 1

	cases := []struct {
		name string
		in   any
		want bool
	}{
		{name: "nil", in: nil, want: false},

		// int family
		{name: "int zero", in: 0, want: false},
		{name: "int positive", in: 1, want: true},
		{name: "int negative", in: -1, want: true},
		{name: "int pointer zero", in: txPtr(0), want: false},
		{name: "int pointer positive", in: txPtr(1), want: true},
		{name: "int pointer nil", in: (*int)(nil), want: false},
		{name: "int8 zero", in: int8(0), want: false},
		{name: "int8 positive", in: int8(1), want: true},
		{name: "int8 pointer zero", in: txPtr(int8(0)), want: false},
		{name: "int8 pointer positive", in: txPtr(int8(1)), want: true},
		{name: "int8 pointer nil", in: (*int8)(nil), want: false},
		{name: "int16 zero", in: int16(0), want: false},
		{name: "int16 positive", in: int16(1), want: true},
		{name: "int16 pointer zero", in: txPtr(int16(0)), want: false},
		{name: "int16 pointer positive", in: txPtr(int16(1)), want: true},
		{name: "int16 pointer nil", in: (*int16)(nil), want: false},
		{name: "int32 zero", in: int32(0), want: false},
		{name: "int32 positive", in: int32(1), want: true},
		{name: "int32 pointer zero", in: txPtr(int32(0)), want: false},
		{name: "int32 pointer positive", in: txPtr(int32(1)), want: true},
		{name: "int32 pointer nil", in: (*int32)(nil), want: false},
		{name: "int64 zero", in: int64(0), want: false},
		{name: "int64 positive", in: int64(1), want: true},
		{name: "int64 pointer zero", in: txPtr(int64(0)), want: false},
		{name: "int64 pointer positive", in: txPtr(int64(1)), want: true},
		{name: "int64 pointer nil", in: (*int64)(nil), want: false},

		// uint family
		{name: "uint zero", in: uint(0), want: false},
		{name: "uint positive", in: uint(1), want: true},
		{name: "uint pointer zero", in: txPtr(uint(0)), want: false},
		{name: "uint pointer positive", in: txPtr(uint(1)), want: true},
		{name: "uint pointer nil", in: (*uint)(nil), want: false},
		{name: "uint8 zero", in: uint8(0), want: false},
		{name: "uint8 positive", in: uint8(1), want: true},
		{name: "uint8 pointer zero", in: txPtr(uint8(0)), want: false},
		{name: "uint8 pointer positive", in: txPtr(uint8(1)), want: true},
		{name: "uint8 pointer nil", in: (*uint8)(nil), want: false},
		{name: "uint16 zero", in: uint16(0), want: false},
		{name: "uint16 positive", in: uint16(1), want: true},
		{name: "uint16 pointer zero", in: txPtr(uint16(0)), want: false},
		{name: "uint16 pointer positive", in: txPtr(uint16(1)), want: true},
		{name: "uint16 pointer nil", in: (*uint16)(nil), want: false},
		{name: "uint32 zero", in: uint32(0), want: false},
		{name: "uint32 positive", in: uint32(1), want: true},
		{name: "uint32 pointer zero", in: txPtr(uint32(0)), want: false},
		{name: "uint32 pointer positive", in: txPtr(uint32(1)), want: true},
		{name: "uint32 pointer nil", in: (*uint32)(nil), want: false},
		{name: "uint64 zero", in: uint64(0), want: false},
		{name: "uint64 positive", in: uint64(1), want: true},
		{name: "uint64 pointer zero", in: txPtr(uint64(0)), want: false},
		{name: "uint64 pointer positive", in: txPtr(uint64(1)), want: true},
		{name: "uint64 pointer nil", in: (*uint64)(nil), want: false},

		// float family
		{name: "float32 zero", in: float32(0), want: false},
		{name: "float32 positive", in: float32(1), want: true},
		{name: "float32 pointer zero", in: txPtr(float32(0)), want: false},
		{name: "float32 pointer positive", in: txPtr(float32(1)), want: true},
		{name: "float32 pointer nil", in: (*float32)(nil), want: false},
		{name: "float64 zero", in: float64(0), want: false},
		{name: "float64 positive", in: float64(1), want: true},
		{name: "float64 pointer zero", in: txPtr(float64(0)), want: false},
		{name: "float64 pointer positive", in: txPtr(float64(1)), want: true},
		{name: "float64 pointer nil", in: (*float64)(nil), want: false},

		// complex values are stringified, so a zero complex is still true.
		{name: "complex64 zero", in: complex64(0), want: true},
		{name: "complex64 nonzero", in: complex64(1 + 2i), want: true},
		{name: "complex64 pointer zero", in: txPtr(complex64(0)), want: true},
		{name: "complex64 pointer nil", in: (*complex64)(nil), want: false},
		{name: "complex128 zero", in: complex128(0), want: true},
		{name: "complex128 nonzero", in: complex128(1 + 2i), want: true},
		{name: "complex128 pointer nonzero", in: txPtr(complex128(1 + 2i)), want: true},
		{name: "complex128 pointer nil", in: (*complex128)(nil), want: false},

		// bool
		{name: "bool true", in: true, want: true},
		{name: "bool false", in: false, want: false},
		{name: "bool pointer true", in: txPtr(true), want: true},
		{name: "bool pointer false", in: txPtr(false), want: false},
		{name: "bool pointer nil", in: (*bool)(nil), want: false},

		// string
		{name: "empty string", in: "", want: false},
		{name: "zero string", in: "0", want: false},
		{name: "false string", in: "false", want: false},
		{name: "upper false string", in: "FALSE", want: false},
		{name: "mixed false string", in: "False", want: false},
		{name: "one string", in: "1", want: true},
		{name: "true string", in: "true", want: true},
		{name: "other string", in: "yes", want: true},
		{name: "string pointer zero", in: txPtr("0"), want: false},
		{name: "string pointer true", in: txPtr("true"), want: true},
		{name: "string pointer nil", in: (*string)(nil), want: false},

		// byte slice
		{name: "empty bytes", in: []byte(""), want: false},
		{name: "zero bytes", in: []byte("0"), want: false},
		{name: "false bytes", in: []byte("false"), want: false},
		{name: "nonempty bytes", in: []byte("x"), want: true},
		{name: "nil bytes", in: []byte(nil), want: false},
		{name: "bytes pointer zero", in: txPtr([]byte("0")), want: false},
		{name: "bytes pointer nonempty", in: txPtr([]byte("x")), want: true},
		{name: "bytes pointer nil", in: (*[]byte)(nil), want: false},

		// time
		{name: "zero time", in: time.Time{}, want: false},
		{name: "non-zero time", in: time.Unix(1, 0), want: true},
		{name: "time pointer zero", in: txPtr(time.Time{}), want: false},
		{name: "time pointer non-zero", in: txPtr(time.Unix(1, 0)), want: true},
		{name: "time pointer nil", in: (*time.Time)(nil), want: false},

		// reflection fallback
		{name: "named bool true", in: txBool(true), want: true},
		{name: "named bool false", in: txBool(false), want: false},
		{name: "named string empty", in: txStr(""), want: false},
		{name: "named string false", in: txStr("false"), want: false},
		{name: "named string true", in: txStr("true"), want: true},
		{name: "named int zero", in: txInt(0), want: false},
		{name: "named int positive", in: txInt(1), want: true},
		{name: "named int8 positive", in: txInt8(1), want: true},
		{name: "named int64 positive", in: txInt64(1), want: true},
		{name: "named uint positive", in: txUint(1), want: true},
		{name: "named uint8 zero", in: txUint8(0), want: false},
		{name: "named uint64 positive", in: txUint64(1), want: true},
		{name: "named float32 zero", in: txFloat32(0), want: false},
		{name: "named float64 positive", in: txFloat64(1), want: true},
		{name: "named complex64 zero", in: txComplex64(0), want: true},
		{name: "named complex128 nonzero", in: txComplex128(1 + 2i), want: true},
		{name: "uintptr zero", in: uintptr(0), want: false},
		{name: "uintptr positive", in: uintptr(1), want: true},
		{name: "nil unsafe pointer", in: unsafe.Pointer(nil), want: false},
		{name: "unsafe pointer", in: unsafe.Pointer(&n), want: true},
		{name: "zero length array", in: [0]int{}, want: false},
		{name: "array", in: [2]int{}, want: true},
		{name: "nil slice", in: []int(nil), want: false},
		{name: "empty slice", in: []int{}, want: false},
		{name: "nonempty slice", in: []int{0}, want: true},
		{name: "nil map", in: map[string]int(nil), want: false},
		{name: "empty map", in: map[string]int{}, want: false},
		{name: "nonempty map", in: map[string]int{"a": 1}, want: true},
		{name: "empty struct", in: struct{}{}, want: true},
		{name: "struct", in: struct{ A int }{A: 1}, want: true},
		{name: "nil chan", in: (chan int)(nil), want: false},
		{name: "chan", in: make(chan int), want: true},
		{name: "nil func", in: (func())(nil), want: false},
		{name: "func", in: func() {}, want: true},
		{name: "interface pointer non-nil", in: txPtr[any](1), want: true},
		{name: "interface pointer nil", in: txPtr[any](nil), want: false},
		{name: "nil struct pointer", in: (*struct{})(nil), want: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Bool(tt.in); got != tt.want {
				t.Errorf("Bool(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestBools_Exhaustive feeds every supported slice type into Bools.
func TestBools_Exhaustive(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []bool
	}{
		{name: "nil", in: nil, want: nil},
		{name: "int slice", in: []int{0, 1, 2}, want: []bool{false, true, true}},
		{name: "int slice pointer", in: txPtr([]int{0, 1}), want: []bool{false, true}},
		{name: "int slice pointer nil", in: (*[]int)(nil), want: nil},
		{name: "int8 slice", in: []int8{0, 1}, want: []bool{false, true}},
		{name: "int8 slice pointer", in: txPtr([]int8{0, 1}), want: []bool{false, true}},
		{name: "int8 slice pointer nil", in: (*[]int8)(nil), want: nil},
		{name: "int16 slice", in: []int16{0, 1}, want: []bool{false, true}},
		{name: "int16 slice pointer", in: txPtr([]int16{0, 1}), want: []bool{false, true}},
		{name: "int16 slice pointer nil", in: (*[]int16)(nil), want: nil},
		{name: "int32 slice", in: []int32{0, 1}, want: []bool{false, true}},
		{name: "int32 slice pointer", in: txPtr([]int32{0, 1}), want: []bool{false, true}},
		{name: "int32 slice pointer nil", in: (*[]int32)(nil), want: nil},
		{name: "int64 slice", in: []int64{0, 1}, want: []bool{false, true}},
		{name: "int64 slice pointer", in: txPtr([]int64{0, 1}), want: []bool{false, true}},
		{name: "int64 slice pointer nil", in: (*[]int64)(nil), want: nil},
		{name: "uint slice", in: []uint{0, 1}, want: []bool{false, true}},
		{name: "uint slice pointer", in: txPtr([]uint{0, 1}), want: []bool{false, true}},
		{name: "uint slice pointer nil", in: (*[]uint)(nil), want: nil},
		{name: "uint8 slice", in: []uint8{0, 1}, want: []bool{false, true}},
		{name: "uint8 slice pointer", in: txPtr([]uint8{0, 1}), want: []bool{false, true}},
		{name: "uint8 slice pointer nil", in: (*[]uint8)(nil), want: nil},
		{name: "uint16 slice", in: []uint16{0, 1}, want: []bool{false, true}},
		{name: "uint16 slice pointer", in: txPtr([]uint16{0, 1}), want: []bool{false, true}},
		{name: "uint16 slice pointer nil", in: (*[]uint16)(nil), want: nil},
		{name: "uint32 slice", in: []uint32{0, 1}, want: []bool{false, true}},
		{name: "uint32 slice pointer", in: txPtr([]uint32{0, 1}), want: []bool{false, true}},
		{name: "uint32 slice pointer nil", in: (*[]uint32)(nil), want: nil},
		{name: "uint64 slice", in: []uint64{0, 1}, want: []bool{false, true}},
		{name: "uint64 slice pointer", in: txPtr([]uint64{0, 1}), want: []bool{false, true}},
		{name: "uint64 slice pointer nil", in: (*[]uint64)(nil), want: nil},
		{name: "float32 slice", in: []float32{0, 1}, want: []bool{false, true}},
		{name: "float32 slice pointer", in: txPtr([]float32{0, 1}), want: []bool{false, true}},
		{name: "float32 slice pointer nil", in: (*[]float32)(nil), want: nil},
		{name: "float64 slice", in: []float64{0, 1}, want: []bool{false, true}},
		{name: "float64 slice pointer", in: txPtr([]float64{0, 1}), want: []bool{false, true}},
		{name: "float64 slice pointer nil", in: (*[]float64)(nil), want: nil},
		{name: "complex64 slice", in: []complex64{complex64(0), complex64(1 + 2i)}, want: []bool{true, true}},
		{name: "complex64 slice pointer", in: txPtr([]complex64{complex64(0)}), want: []bool{true}},
		{name: "complex64 slice pointer nil", in: (*[]complex64)(nil), want: nil},
		{name: "complex128 slice", in: []complex128{complex128(0), complex128(1 + 2i)}, want: []bool{true, true}},
		{name: "complex128 slice pointer", in: txPtr([]complex128{complex128(0)}), want: []bool{true}},
		{name: "complex128 slice pointer nil", in: (*[]complex128)(nil), want: nil},
		{name: "string slice", in: []string{"1", "0", "false", "", "x"}, want: []bool{true, false, false, false, true}},
		{name: "string slice pointer", in: txPtr([]string{"1", "0"}), want: []bool{true, false}},
		{name: "string slice pointer nil", in: (*[]string)(nil), want: nil},
		{name: "bool slice", in: []bool{true, false}, want: []bool{true, false}},
		{name: "bool slice pointer", in: txPtr([]bool{true, false}), want: []bool{true, false}},
		{name: "bool slice pointer nil", in: (*[]bool)(nil), want: nil},
		{name: "any slice", in: []any{1, "0", nil, true}, want: []bool{true, false, false, true}},
		{name: "any slice pointer", in: txPtr([]any{1, "0"}), want: []bool{true, false}},
		{name: "any slice pointer nil", in: (*[]any)(nil), want: nil},
		{name: "bytes slice", in: [][]byte{[]byte("1"), []byte("0")}, want: []bool{true, false}},
		{name: "bytes slice pointer", in: txPtr([][]byte{[]byte("1")}), want: []bool{true}},
		{name: "bytes slice pointer nil", in: (*[][]byte)(nil), want: nil},
		{name: "named int slice", in: txIntSlice{0, 1}, want: []bool{false, true}},
		{name: "named bool slice", in: txBoolSlice{true, false}, want: []bool{true, false}},
		{name: "array", in: [2]int{0, 1}, want: []bool{false, true}},
		{name: "empty int slice", in: []int{}, want: []bool{}},
		{name: "unsupported int", in: 123, want: nil},
		{name: "unsupported string", in: "abc", want: nil},
		{name: "unsupported struct", in: struct{}{}, want: nil},
		{name: "unsupported func", in: func() {}, want: nil},
		{name: "nil struct pointer", in: (*struct{})(nil), want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Bools(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bools(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestString_Exhaustive feeds every supported input type into String.
func TestString_Exhaustive(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{name: "nil", in: nil, want: ""},

		// int family
		{name: "int", in: 123, want: "123"},
		{name: "int negative", in: -8, want: "-8"},
		{name: "int pointer", in: txPtr(123), want: "123"},
		{name: "int pointer nil", in: (*int)(nil), want: ""},
		{name: "int8", in: int8(-8), want: "-8"},
		{name: "int8 pointer", in: txPtr(int8(-8)), want: "-8"},
		{name: "int8 pointer nil", in: (*int8)(nil), want: ""},
		{name: "int16", in: int16(-16), want: "-16"},
		{name: "int16 pointer", in: txPtr(int16(-16)), want: "-16"},
		{name: "int16 pointer nil", in: (*int16)(nil), want: ""},
		{name: "int32", in: int32(-32), want: "-32"},
		{name: "int32 pointer", in: txPtr(int32(-32)), want: "-32"},
		{name: "int32 pointer nil", in: (*int32)(nil), want: ""},
		{name: "int64", in: int64(-64), want: "-64"},
		{name: "int64 pointer", in: txPtr(int64(-64)), want: "-64"},
		{name: "int64 pointer nil", in: (*int64)(nil), want: ""},

		// uint family
		{name: "uint", in: uint(42), want: "42"},
		{name: "uint pointer", in: txPtr(uint(42)), want: "42"},
		{name: "uint pointer nil", in: (*uint)(nil), want: ""},
		{name: "uint8", in: uint8(255), want: "255"},
		{name: "uint8 pointer", in: txPtr(uint8(255)), want: "255"},
		{name: "uint8 pointer nil", in: (*uint8)(nil), want: ""},
		{name: "uint16", in: uint16(16), want: "16"},
		{name: "uint16 pointer", in: txPtr(uint16(16)), want: "16"},
		{name: "uint16 pointer nil", in: (*uint16)(nil), want: ""},
		{name: "uint32", in: uint32(32), want: "32"},
		{name: "uint32 pointer", in: txPtr(uint32(32)), want: "32"},
		{name: "uint32 pointer nil", in: (*uint32)(nil), want: ""},
		{name: "uint64", in: uint64(64), want: "64"},
		{name: "uint64 pointer", in: txPtr(uint64(64)), want: "64"},
		{name: "uint64 pointer nil", in: (*uint64)(nil), want: ""},

		// float family
		{name: "float32 shortest", in: float32(1.5), want: "1.5"},
		{name: "float32 pointer", in: txPtr(float32(1.5)), want: "1.5"},
		{name: "float32 pointer nil", in: (*float32)(nil), want: ""},
		{name: "float64 shortest", in: 3.14, want: "3.14"},
		{name: "float64 pointer", in: txPtr(3.14), want: "3.14"},
		{name: "float64 pointer nil", in: (*float64)(nil), want: ""},

		// complex family
		{name: "complex64", in: complex64(1 + 2i), want: "(1e+00+2e+00i)"},
		{name: "complex64 pointer", in: txPtr(complex64(1 + 2i)), want: "(1e+00+2e+00i)"},
		{name: "complex64 pointer nil", in: (*complex64)(nil), want: ""},
		{name: "complex128", in: complex128(1 + 2i), want: "(1e+00+2e+00i)"},
		{name: "complex128 pointer", in: txPtr(complex128(1 + 2i)), want: "(1e+00+2e+00i)"},
		{name: "complex128 pointer nil", in: (*complex128)(nil), want: ""},

		// bool
		{name: "bool true", in: true, want: "true"},
		{name: "bool false", in: false, want: "false"},
		{name: "bool pointer true", in: txPtr(true), want: "true"},
		{name: "bool pointer false", in: txPtr(false), want: "false"},
		{name: "bool pointer nil", in: (*bool)(nil), want: ""},

		// string
		{name: "string", in: "hello", want: "hello"},
		{name: "string pointer", in: txPtr("hello"), want: "hello"},
		{name: "string pointer nil", in: (*string)(nil), want: ""},

		// byte slice
		{name: "bytes", in: []byte("go"), want: "go"},
		{name: "bytes pointer", in: txPtr([]byte("go")), want: "go"},
		{name: "bytes pointer nil", in: (*[]byte)(nil), want: ""},
		{name: "nil bytes", in: []byte(nil), want: ""},

		// time
		{name: "zero time", in: time.Time{}, want: ""},
		{name: "time", in: time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC), want: "2023-01-02 03:04:05 +0000 UTC"},
		{name: "time pointer zero", in: txPtr(time.Time{}), want: ""},
		{name: "time pointer", in: txPtr(time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC)), want: "2023-01-02 03:04:05 +0000 UTC"},
		{name: "time pointer nil", in: (*time.Time)(nil), want: ""},

		// reflection fallback
		{name: "named string", in: txStr("x"), want: "x"},
		{name: "named int", in: txInt(9), want: "9"},
		{name: "named int8", in: txInt8(-9), want: "-9"},
		{name: "named int64", in: txInt64(9), want: "9"},
		{name: "named uint", in: txUint(9), want: "9"},
		{name: "named uint8", in: txUint8(9), want: "9"},
		{name: "named uint64", in: txUint64(9), want: "9"},
		{name: "named float32", in: txFloat32(1.5), want: "1.5"},
		{name: "named float64", in: txFloat64(1.5), want: "1.5"},
		{name: "named complex64", in: txComplex64(1 + 2i), want: "(1e+00+2e+00i)"},
		{name: "named complex128", in: txComplex128(1 + 2i), want: "(1e+00+2e+00i)"},
		{name: "named bool", in: txBool(true), want: "true"},
		{name: "uintptr", in: uintptr(7), want: "7"},
		{name: "array json", in: [3]int{1, 2, 3}, want: "[1,2,3]"},
		{name: "int slice json", in: []int{1, 2}, want: "[1,2]"},
		{name: "named int slice json", in: txIntSlice{1, 2}, want: "[1,2]"},
		{name: "map json", in: map[string]int{"a": 1}, want: `{"a":1}`},
		{name: "struct json", in: struct {
			ID int `json:"id"`
		}{ID: 7}, want: `{"id":7}`},
		{name: "nil func", in: (func())(nil), want: "<nil>"},
		{name: "nil chan", in: (chan int)(nil), want: "<nil>"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.String(tt.in); got != tt.want {
				t.Errorf("String(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}

	// Non-nil func, chan and unsafe pointer values fall back to fmt.Sprintf and
	// yield an address that cannot be asserted exactly.
	t.Run("non-deterministic fallback", func(t *testing.T) {
		n := 1
		inputs := []struct {
			name string
			in   any
		}{
			{name: "func", in: func() {}},
			{name: "chan", in: make(chan int)},
			{name: "unsafe pointer", in: unsafe.Pointer(&n)},
			{name: "struct with chan", in: txChanHolder{C: make(chan int)}},
		}

		for _, tt := range inputs {
			t.Run(tt.name, func(t *testing.T) {
				if got := xconv.String(tt.in); got == "" {
					t.Errorf("String(%v) = %q, want a non-empty fallback", tt.in, got)
				}
			})
		}
	})
}

// TestStrings_Exhaustive feeds every supported slice type into Strings.
func TestStrings_Exhaustive(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []string
	}{
		{name: "nil", in: nil, want: nil},
		{name: "int slice", in: []int{1, 2}, want: []string{"1", "2"}},
		{name: "int slice pointer", in: txPtr([]int{1}), want: []string{"1"}},
		{name: "int slice pointer nil", in: (*[]int)(nil), want: nil},
		{name: "int8 slice", in: []int8{-8}, want: []string{"-8"}},
		{name: "int8 slice pointer", in: txPtr([]int8{-8}), want: []string{"-8"}},
		{name: "int8 slice pointer nil", in: (*[]int8)(nil), want: nil},
		{name: "int16 slice", in: []int16{16}, want: []string{"16"}},
		{name: "int16 slice pointer", in: txPtr([]int16{16}), want: []string{"16"}},
		{name: "int16 slice pointer nil", in: (*[]int16)(nil), want: nil},
		{name: "int32 slice", in: []int32{32}, want: []string{"32"}},
		{name: "int32 slice pointer", in: txPtr([]int32{32}), want: []string{"32"}},
		{name: "int32 slice pointer nil", in: (*[]int32)(nil), want: nil},
		{name: "int64 slice", in: []int64{64}, want: []string{"64"}},
		{name: "int64 slice pointer", in: txPtr([]int64{64}), want: []string{"64"}},
		{name: "int64 slice pointer nil", in: (*[]int64)(nil), want: nil},
		{name: "uint slice", in: []uint{1}, want: []string{"1"}},
		{name: "uint slice pointer", in: txPtr([]uint{1}), want: []string{"1"}},
		{name: "uint slice pointer nil", in: (*[]uint)(nil), want: nil},
		{name: "uint8 slice", in: []uint8{65}, want: []string{"65"}},
		{name: "uint8 slice pointer", in: txPtr([]uint8{65}), want: []string{"65"}},
		{name: "uint8 slice pointer nil", in: (*[]uint8)(nil), want: nil},
		{name: "uint16 slice", in: []uint16{16}, want: []string{"16"}},
		{name: "uint16 slice pointer", in: txPtr([]uint16{16}), want: []string{"16"}},
		{name: "uint16 slice pointer nil", in: (*[]uint16)(nil), want: nil},
		{name: "uint32 slice", in: []uint32{32}, want: []string{"32"}},
		{name: "uint32 slice pointer", in: txPtr([]uint32{32}), want: []string{"32"}},
		{name: "uint32 slice pointer nil", in: (*[]uint32)(nil), want: nil},
		{name: "uint64 slice", in: []uint64{64}, want: []string{"64"}},
		{name: "uint64 slice pointer", in: txPtr([]uint64{64}), want: []string{"64"}},
		{name: "uint64 slice pointer nil", in: (*[]uint64)(nil), want: nil},
		{name: "float32 slice", in: []float32{1.5}, want: []string{"1.5"}},
		{name: "float32 slice pointer", in: txPtr([]float32{1.5}), want: []string{"1.5"}},
		{name: "float32 slice pointer nil", in: (*[]float32)(nil), want: nil},
		{name: "float64 slice", in: []float64{1.5, 2}, want: []string{"1.5", "2"}},
		{name: "float64 slice pointer", in: txPtr([]float64{1.5}), want: []string{"1.5"}},
		{name: "float64 slice pointer nil", in: (*[]float64)(nil), want: nil},
		{name: "complex64 slice", in: []complex64{complex64(1 + 2i)}, want: []string{"(1e+00+2e+00i)"}},
		{name: "complex64 slice pointer", in: txPtr([]complex64{complex64(1 + 2i)}), want: []string{"(1e+00+2e+00i)"}},
		{name: "complex64 slice pointer nil", in: (*[]complex64)(nil), want: nil},
		{name: "complex128 slice", in: []complex128{complex128(1 + 2i)}, want: []string{"(1e+00+2e+00i)"}},
		{name: "complex128 slice pointer", in: txPtr([]complex128{complex128(1 + 2i)}), want: []string{"(1e+00+2e+00i)"}},
		{name: "complex128 slice pointer nil", in: (*[]complex128)(nil), want: nil},
		{name: "string slice", in: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "string slice pointer", in: txPtr([]string{"a"}), want: []string{"a"}},
		{name: "string slice pointer nil", in: (*[]string)(nil), want: nil},
		{name: "bool slice", in: []bool{true, false}, want: []string{"true", "false"}},
		{name: "bool slice pointer", in: txPtr([]bool{true}), want: []string{"true"}},
		{name: "bool slice pointer nil", in: (*[]bool)(nil), want: nil},
		{name: "any slice", in: []any{1, "x", true, nil}, want: []string{"1", "x", "true", ""}},
		{name: "any slice pointer", in: txPtr([]any{1}), want: []string{"1"}},
		{name: "any slice pointer nil", in: (*[]any)(nil), want: nil},
		{name: "bytes slice", in: [][]byte{[]byte("a"), []byte("b")}, want: []string{"a", "b"}},
		{name: "bytes slice pointer", in: txPtr([][]byte{[]byte("a")}), want: []string{"a"}},
		{name: "bytes slice pointer nil", in: (*[][]byte)(nil), want: nil},
		{name: "named int slice", in: txIntSlice{1, 2}, want: []string{"1", "2"}},
		{name: "named string slice", in: txStrSlice{"a"}, want: []string{"a"}},
		{name: "array", in: [3]int{1, 2, 3}, want: []string{"1", "2", "3"}},
		{name: "empty string slice", in: []string{}, want: []string{}},
		{name: "unsupported int", in: 123, want: nil},
		{name: "unsupported string", in: "abc", want: nil},
		{name: "unsupported struct", in: struct{}{}, want: nil},
		{name: "unsupported func", in: func() {}, want: nil},
		{name: "nil struct pointer", in: (*struct{})(nil), want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Strings(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Strings(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestByte_Exhaustive feeds every supported input type into Byte.
func TestByte_Exhaustive(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want byte
	}{
		{name: "nil", in: nil, want: 0},
		{name: "int", in: 65, want: 65},
		{name: "negative int", in: -1, want: 255},
		{name: "overflow int", in: 256, want: 0},
		{name: "int pointer", in: txPtr(65), want: 65},
		{name: "int pointer nil", in: (*int)(nil), want: 0},
		{name: "int8", in: int8(65), want: 65},
		{name: "int16", in: int16(65), want: 65},
		{name: "int32", in: int32(200), want: 200},
		{name: "int64", in: int64(65), want: 65},
		{name: "uint", in: uint(65), want: 65},
		{name: "uint8", in: uint8(200), want: 200},
		{name: "uint16", in: uint16(65), want: 65},
		{name: "uint32", in: uint32(65), want: 65},
		{name: "uint64", in: uint64(65), want: 65},
		{name: "float32", in: float32(1.9), want: 1},
		{name: "float64", in: 1.9, want: 1},
		{name: "complex64", in: complex64(1 + 2i), want: 1},
		{name: "complex128", in: complex128(1 + 2i), want: 1},
		{name: "bool true", in: true, want: 1},
		{name: "bool false", in: false, want: 0},
		{name: "numeric string", in: "65", want: 65},
		{name: "invalid string", in: "abc", want: 0},
		{name: "bytes", in: []byte{65}, want: 65},
		{name: "time", in: time.Unix(0, 65), want: 65},
		{name: "named int", in: txInt(65), want: 65},
		{name: "named string", in: txStr("65"), want: 65},
		{name: "uintptr", in: uintptr(65), want: 65},
		{name: "unsupported struct", in: struct{}{}, want: 0},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Byte(tt.in); got != tt.want {
				t.Errorf("Byte(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestBytes_Exhaustive feeds every supported input type into Bytes.
func TestBytes_Exhaustive(t *testing.T) {
	eight := func(v byte) []byte { return []byte{0, 0, 0, 0, 0, 0, 0, v} }

	cases := []struct {
		name string
		in   any
		want []byte
	}{
		{name: "nil", in: nil, want: nil},

		// int family
		{name: "int", in: 1, want: eight(1)},
		{name: "int pointer", in: txPtr(1), want: eight(1)},
		{name: "int pointer nil", in: (*int)(nil), want: nil},
		{name: "int8", in: int8(1), want: []byte{1}},
		{name: "int8 pointer", in: txPtr(int8(1)), want: []byte{1}},
		{name: "int8 pointer nil", in: (*int8)(nil), want: nil},
		{name: "int16", in: int16(1), want: []byte{0, 1}},
		{name: "int16 pointer", in: txPtr(int16(1)), want: []byte{0, 1}},
		{name: "int16 pointer nil", in: (*int16)(nil), want: nil},
		{name: "int32", in: int32(1), want: []byte{0, 0, 0, 1}},
		{name: "int32 pointer", in: txPtr(int32(1)), want: []byte{0, 0, 0, 1}},
		{name: "int32 pointer nil", in: (*int32)(nil), want: nil},
		{name: "int64", in: int64(1), want: eight(1)},
		{name: "int64 pointer", in: txPtr(int64(1)), want: eight(1)},
		{name: "int64 pointer nil", in: (*int64)(nil), want: nil},

		// uint family
		{name: "uint", in: uint(1), want: eight(1)},
		{name: "uint pointer", in: txPtr(uint(1)), want: eight(1)},
		{name: "uint pointer nil", in: (*uint)(nil), want: nil},
		{name: "uint8", in: uint8(255), want: []byte{255}},
		{name: "uint8 pointer", in: txPtr(uint8(255)), want: []byte{255}},
		{name: "uint8 pointer nil", in: (*uint8)(nil), want: nil},
		{name: "uint16", in: uint16(1), want: []byte{0, 1}},
		{name: "uint16 pointer", in: txPtr(uint16(1)), want: []byte{0, 1}},
		{name: "uint16 pointer nil", in: (*uint16)(nil), want: nil},
		{name: "uint32", in: uint32(1), want: []byte{0, 0, 0, 1}},
		{name: "uint32 pointer", in: txPtr(uint32(1)), want: []byte{0, 0, 0, 1}},
		{name: "uint32 pointer nil", in: (*uint32)(nil), want: nil},
		{name: "uint64", in: uint64(1), want: eight(1)},
		{name: "uint64 pointer", in: txPtr(uint64(1)), want: eight(1)},
		{name: "uint64 pointer nil", in: (*uint64)(nil), want: nil},

		// float family
		{name: "float32", in: float32(1.5), want: []byte{0x3f, 0xc0, 0x00, 0x00}},
		{name: "float32 pointer", in: txPtr(float32(1.5)), want: []byte{0x3f, 0xc0, 0x00, 0x00}},
		{name: "float32 pointer nil", in: (*float32)(nil), want: nil},
		{name: "float64", in: float64(1.5), want: []byte{0x3f, 0xf8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{name: "float64 pointer", in: txPtr(float64(1.5)), want: []byte{0x3f, 0xf8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{name: "float64 pointer nil", in: (*float64)(nil), want: nil},

		// bool
		{name: "bool true", in: true, want: []byte{1}},
		{name: "bool false", in: false, want: []byte{0}},
		{name: "bool pointer true", in: txPtr(true), want: []byte{1}},
		{name: "bool pointer nil", in: (*bool)(nil), want: nil},

		// uintptr
		{name: "uintptr", in: uintptr(1), want: eight(1)},
		{name: "uintptr pointer", in: txPtr(uintptr(1)), want: eight(1)},
		{name: "uintptr pointer nil", in: (*uintptr)(nil), want: nil},

		// complex
		{name: "complex64", in: complex64(1 + 2i), want: nil},
		{name: "complex64 pointer", in: txPtr(complex64(1 + 2i)), want: nil},
		{name: "complex128", in: complex128(1 + 2i), want: nil},
		{name: "complex128 pointer", in: txPtr(complex128(1 + 2i)), want: nil},

		// string
		{name: "string", in: "1", want: []byte{0x31}},
		{name: "long string", in: "go", want: []byte("go")},
		{name: "string pointer", in: txPtr("go"), want: []byte("go")},
		{name: "string pointer nil", in: (*string)(nil), want: nil},

		// byte slice
		{name: "bytes", in: []byte{1, 2, 3}, want: []byte{1, 2, 3}},
		{name: "nil bytes", in: []byte(nil), want: nil},
		{name: "bytes pointer", in: txPtr([]byte{1, 2, 3}), want: []byte{1, 2, 3}},
		{name: "bytes pointer nil", in: (*[]byte)(nil), want: nil},

		// reflection fallback
		{name: "named int", in: txInt(1), want: eight(1)},
		{name: "named int8", in: txInt8(1), want: []byte{1}},
		{name: "named int16", in: txInt16(1), want: []byte{0, 1}},
		{name: "named int32", in: txInt32(1), want: []byte{0, 0, 0, 1}},
		{name: "named int64", in: txInt64(1), want: eight(1)},
		{name: "named uint", in: txUint(1), want: eight(1)},
		{name: "named uint8", in: txUint8(1), want: []byte{1}},
		{name: "named uint16", in: txUint16(1), want: []byte{0, 1}},
		{name: "named uint32", in: txUint32(1), want: []byte{0, 0, 0, 1}},
		{name: "named uint64", in: txUint64(1), want: eight(1)},
		// A named float32 is written through the reflection path as a float64.
		{name: "named float32", in: txFloat32(1.5), want: []byte{0x3f, 0xf8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{name: "named float64", in: txFloat64(1.5), want: []byte{0x3f, 0xf8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{name: "named bool", in: txBool(true), want: []byte{1}},
		{name: "named string", in: txStr("1"), want: []byte{0x31}},
		{name: "named complex64", in: txComplex64(1 + 2i), want: nil},
		{name: "named complex128", in: txComplex128(1 + 2i), want: nil},

		// JSON fallback
		{name: "array json", in: [3]int{1, 2, 3}, want: []byte("[1,2,3]")},
		{name: "int slice json", in: []int{1, 2}, want: []byte("[1,2]")},
		{name: "named int slice json", in: txIntSlice{1, 2}, want: []byte("[1,2]")},
		{name: "map json", in: map[string]int{"a": 1}, want: []byte(`{"a":1}`)},
		{name: "struct json", in: struct {
			ID int `json:"id"`
		}{ID: 7}, want: []byte(`{"id":7}`)},
		{name: "json marshal failure", in: txChanHolder{C: make(chan int)}, want: nil},
		{name: "nil struct pointer", in: (*struct{})(nil), want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Bytes(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bytes(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
