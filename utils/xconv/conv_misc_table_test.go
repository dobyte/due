package xconv_test

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/dobyte/due/v2/utils/xconv"
)

// miscInt is a named integer type used to exercise the reflection branches of the conversions.
type miscInt int

// miscFloat is a named floating-point type used to exercise the reflection branches of the conversions.
type miscFloat float64

// ptrTo returns a pointer to a copy of v.
func ptrTo[T any](v T) *T {
	return &v
}

// TestString_Table verifies String for scalar, pointer, time and fallback inputs.
func TestString_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "int", in: 123, want: "123"},
		{name: "int8 negative", in: int8(-8), want: "-8"},
		{name: "uint32", in: uint32(42), want: "42"},
		{name: "int pointer", in: ptrTo(7), want: "7"},
		{name: "nil int pointer", in: (*int)(nil), want: ""},
		{name: "named int", in: miscInt(9), want: "9"},
		{name: "float32 shortest", in: float32(1.5), want: "1.5"},
		{name: "float64 shortest", in: 3.14, want: "3.14"},
		{name: "bool true", in: true, want: "true"},
		{name: "bool false", in: false, want: "false"},
		{name: "nil bool pointer", in: (*bool)(nil), want: ""},
		{name: "string", in: "hello", want: "hello"},
		{name: "nil string pointer", in: (*string)(nil), want: ""},
		{name: "bytes", in: []byte("go"), want: "go"},
		{name: "nil bytes", in: []byte(nil), want: ""},
		{name: "nil bytes pointer", in: (*[]byte)(nil), want: ""},
		{name: "zero time", in: time.Time{}, want: ""},
		{name: "time", in: time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC), want: "2023-01-02 03:04:05 +0000 UTC"},
		{name: "nil time pointer", in: (*time.Time)(nil), want: ""},
		{name: "complex64", in: complex64(complex(1, 2)), want: "(1e+00+2e+00i)"},
		{name: "nil func", in: (func())(nil), want: "<nil>"},
		{name: "struct json", in: struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}{ID: 1, Name: "a"}, want: `{"id":1,"name":"a"}`},
		{name: "int slice json", in: []int{1, 2, 3}, want: "[1,2,3]"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.String(tt.in); got != tt.want {
				t.Errorf("String(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestStrings_Table verifies Strings for supported slices, arrays and unsupported inputs.
func TestStrings_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []string
	}{
		{name: "nil", in: nil, want: nil},
		{name: "string slice", in: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "int slice", in: []int{1, 2}, want: []string{"1", "2"}},
		{name: "any slice", in: []any{1, "x", true, nil}, want: []string{"1", "x", "true", ""}},
		{name: "bytes slice", in: [][]byte{[]byte("a"), []byte("b")}, want: []string{"a", "b"}},
		{name: "bool slice", in: []bool{true, false}, want: []string{"true", "false"}},
		{name: "array", in: [3]int{1, 2, 3}, want: []string{"1", "2", "3"}},
		{name: "int slice pointer", in: ptrTo([]int{4, 5}), want: []string{"4", "5"}},
		{name: "nil string slice pointer", in: (*[]string)(nil), want: nil},
		{name: "unsupported", in: 123, want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Strings(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Strings(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestStringPointer_Table verifies StringPointer always returns a non-nil pointer with the converted value.
func TestStringPointer_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "int", in: 5, want: "5"},
		{name: "string", in: "x", want: "x"},
		{name: "bytes", in: []byte("ab"), want: "ab"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.StringPointer(tt.in)
			if got == nil {
				t.Fatalf("StringPointer(%v) = nil, want non-nil pointer", tt.in)
			}
			if *got != tt.want {
				t.Errorf("StringPointer(%v) = %q, want %q", tt.in, *got, tt.want)
			}
		})
	}
}

// TestStringsPointer_Table verifies StringsPointer always returns a non-nil pointer with the converted slice.
func TestStringsPointer_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []string
	}{
		{name: "nil", in: nil, want: nil},
		{name: "int slice", in: []int{1, 2}, want: []string{"1", "2"}},
		{name: "string slice", in: []string{"a"}, want: []string{"a"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.StringsPointer(tt.in)
			if got == nil {
				t.Fatalf("StringsPointer(%v) = nil, want non-nil pointer", tt.in)
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("StringsPointer(%v) = %v, want %v", tt.in, *got, tt.want)
			}
		})
	}
}

// TestBool_Table verifies Bool for numeric, string, time, container and callable inputs.
func TestBool_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want bool
	}{
		{name: "nil", in: nil, want: false},
		{name: "int zero", in: 0, want: false},
		{name: "int nonzero", in: 1, want: true},
		{name: "int negative", in: -5, want: true},
		{name: "uint8 zero", in: uint8(0), want: false},
		{name: "float zero", in: 0.0, want: false},
		{name: "float nonzero", in: 0.5, want: true},
		{name: "nil int pointer", in: (*int)(nil), want: false},
		{name: "int pointer zero", in: ptrTo(0), want: false},
		{name: "int pointer nonzero", in: ptrTo(3), want: true},
		{name: "bool true", in: true, want: true},
		{name: "bool false", in: false, want: false},
		{name: "nil bool pointer", in: (*bool)(nil), want: false},
		{name: "bool pointer false", in: ptrTo(false), want: false},
		{name: "empty string", in: "", want: false},
		{name: "zero string", in: "0", want: false},
		{name: "false string", in: "false", want: false},
		{name: "upper false string", in: "FALSE", want: false},
		{name: "mixed false string", in: "False", want: false},
		{name: "true string", in: "true", want: true},
		{name: "one string", in: "1", want: true},
		{name: "other string", in: "yes", want: true},
		{name: "nil string pointer", in: (*string)(nil), want: false},
		{name: "zero string pointer", in: ptrTo("0"), want: false},
		{name: "true string pointer", in: ptrTo("true"), want: true},
		{name: "zero bytes", in: []byte("0"), want: false},
		{name: "nonempty bytes", in: []byte("x"), want: true},
		{name: "nil bytes pointer", in: (*[]byte)(nil), want: false},
		{name: "zero time", in: time.Time{}, want: false},
		{name: "nonzero time", in: time.Unix(1, 0), want: true},
		{name: "nil time pointer", in: (*time.Time)(nil), want: false},
		{name: "nil slice", in: []int(nil), want: false},
		{name: "empty slice", in: []int{}, want: false},
		{name: "one element slice", in: []int{0}, want: true},
		{name: "nil map", in: map[string]int(nil), want: false},
		{name: "empty map", in: map[string]int{}, want: false},
		{name: "nonempty map", in: map[string]int{"a": 1}, want: true},
		{name: "empty struct", in: struct{}{}, want: true},
		{name: "zero length array", in: [0]int{}, want: false},
		{name: "array", in: [2]int{}, want: true},
		{name: "nil chan", in: (chan int)(nil), want: false},
		{name: "chan", in: make(chan int), want: true},
		{name: "nil func", in: (func())(nil), want: false},
		{name: "func", in: func() {}, want: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Bool(tt.in); got != tt.want {
				t.Errorf("Bool(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestBools_Table verifies Bools for supported slices, arrays and unsupported inputs.
func TestBools_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []bool
	}{
		{name: "nil", in: nil, want: nil},
		{name: "int slice", in: []int{0, 1, 2}, want: []bool{false, true, true}},
		{name: "string slice", in: []string{"1", "0", "false", "", "x"}, want: []bool{true, false, false, false, true}},
		{name: "bool slice", in: []bool{true, false}, want: []bool{true, false}},
		{name: "any slice", in: []any{1, "0", nil, true}, want: []bool{true, false, false, true}},
		{name: "int slice pointer", in: ptrTo([]int{1, 0}), want: []bool{true, false}},
		{name: "nil bool slice pointer", in: (*[]bool)(nil), want: nil},
		{name: "unsupported", in: 123, want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Bools(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bools(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestBoolPointer_Table verifies BoolPointer always returns a non-nil pointer with the converted value.
func TestBoolPointer_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want bool
	}{
		{name: "nil", in: nil, want: false},
		{name: "true string", in: "true", want: true},
		{name: "zero int", in: 0, want: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.BoolPointer(tt.in)
			if got == nil {
				t.Fatalf("BoolPointer(%v) = nil, want non-nil pointer", tt.in)
			}
			if *got != tt.want {
				t.Errorf("BoolPointer(%v) = %v, want %v", tt.in, *got, tt.want)
			}
		})
	}
}

// TestBoolsPointer_Table verifies BoolsPointer always returns a non-nil pointer with the converted slice.
func TestBoolsPointer_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []bool
	}{
		{name: "nil", in: nil, want: nil},
		{name: "int slice", in: []int{1, 0}, want: []bool{true, false}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.BoolsPointer(tt.in)
			if got == nil {
				t.Fatalf("BoolsPointer(%v) = nil, want non-nil pointer", tt.in)
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("BoolsPointer(%v) = %v, want %v", tt.in, *got, tt.want)
			}
		})
	}
}

// TestByte_Table verifies Byte for numeric, string and pointer inputs.
func TestByte_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want byte
	}{
		{name: "nil", in: nil, want: 0},
		{name: "int", in: 65, want: 65},
		{name: "negative int", in: -1, want: 255},
		{name: "overflow int", in: 256, want: 0},
		{name: "numeric string", in: "65", want: 65},
		{name: "invalid string", in: "abc", want: 0},
		{name: "bool true", in: true, want: 1},
		{name: "bool false", in: false, want: 0},
		{name: "float", in: 1.9, want: 1},
		{name: "int32", in: int32(200), want: 200},
		{name: "uint8", in: uint8(7), want: 7},
		{name: "int pointer", in: ptrTo(9), want: 9},
		{name: "nil int pointer", in: (*int)(nil), want: 0},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Byte(tt.in); got != tt.want {
				t.Errorf("Byte(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestBytes_Table verifies the big-endian encoding, zero-copy string handling and JSON fallback of Bytes.
func TestBytes_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []byte
	}{
		{name: "nil", in: nil, want: nil},
		{name: "string", in: "1", want: []byte("1")},
		{name: "long string", in: "hello", want: []byte("hello")},
		{name: "int", in: 1, want: []byte{0, 0, 0, 0, 0, 0, 0, 1}},
		{name: "uint8", in: uint8(255), want: []byte{255}},
		{name: "bool true", in: true, want: []byte{1}},
		{name: "bool false", in: false, want: []byte{0}},
		{name: "int32", in: int32(1), want: []byte{0, 0, 0, 1}},
		{name: "float32", in: float32(1.5), want: []byte{0x3f, 0xc0, 0x00, 0x00}},
		{name: "float64", in: 1.5, want: []byte{0x3f, 0xf8, 0, 0, 0, 0, 0, 0}},
		{name: "bytes", in: []byte{1, 2, 3}, want: []byte{1, 2, 3}},
		{name: "string pointer", in: ptrTo("go"), want: []byte("go")},
		{name: "nil string pointer", in: (*string)(nil), want: nil},
		{name: "nil int pointer", in: (*int)(nil), want: nil},
		{name: "complex", in: complex64(1), want: nil},
		{name: "int slice json", in: []int{1, 2}, want: []byte("[1,2]")},
		{name: "struct json", in: struct {
			ID int `json:"id"`
		}{ID: 7}, want: []byte(`{"id":7}`)},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Bytes(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bytes(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestBytePointer_Table verifies BytePointer always returns a non-nil pointer with the converted value.
func TestBytePointer_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want byte
	}{
		{name: "nil", in: nil, want: 0},
		{name: "int", in: 65, want: 65},
		{name: "string", in: "65", want: 65},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.BytePointer(tt.in)
			if got == nil {
				t.Fatalf("BytePointer(%v) = nil, want non-nil pointer", tt.in)
			}
			if *got != tt.want {
				t.Errorf("BytePointer(%v) = %v, want %v", tt.in, *got, tt.want)
			}
		})
	}
}

// TestBytesPointer_Table verifies BytesPointer always returns a non-nil pointer with the converted slice.
func TestBytesPointer_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []byte
	}{
		{name: "nil", in: nil, want: nil},
		{name: "string", in: "hi", want: []byte("hi")},
		{name: "int", in: 2, want: []byte{0, 0, 0, 0, 0, 0, 0, 2}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.BytesPointer(tt.in)
			if got == nil {
				t.Fatalf("BytesPointer(%v) = nil, want non-nil pointer", tt.in)
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("BytesPointer(%v) = %v, want %v", tt.in, *got, tt.want)
			}
		})
	}
}

// TestRune_Table verifies Rune for numeric, string and pointer inputs.
func TestRune_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want rune
	}{
		{name: "nil", in: nil, want: 0},
		{name: "rune literal", in: 'a', want: 97},
		{name: "int", in: 65, want: 65},
		{name: "numeric string", in: "65", want: 65},
		{name: "non numeric string", in: "a", want: 0},
		{name: "float", in: 65.9, want: 65},
		{name: "uint8", in: uint8(200), want: 200},
		{name: "int pointer", in: ptrTo(70), want: 70},
		{name: "nil int pointer", in: (*int)(nil), want: 0},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Rune(tt.in); got != tt.want {
				t.Errorf("Rune(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestRunes_Table verifies Runes for supported slices and unsupported inputs.
func TestRunes_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []rune
	}{
		{name: "nil", in: nil, want: nil},
		{name: "rune slice", in: []rune("abc"), want: []rune("abc")},
		{name: "int32 slice", in: []int32{1, 2}, want: []int32{1, 2}},
		{name: "string slice", in: []string{"a", "65"}, want: []int32{0, 65}},
		{name: "int slice", in: []int{1, 2, 3}, want: []int32{1, 2, 3}},
		{name: "unsupported string", in: "abc", want: nil},
		{name: "unsupported int", in: 123, want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Runes(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Runes(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestRunePointer_Table verifies RunePointer always returns a non-nil pointer with the converted value.
func TestRunePointer_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int32
	}{
		{name: "nil", in: nil, want: 0},
		{name: "rune literal", in: 'z', want: 122},
		{name: "int", in: 65, want: 65},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.RunePointer(tt.in)
			if got == nil {
				t.Fatalf("RunePointer(%v) = nil, want non-nil pointer", tt.in)
			}
			if *got != tt.want {
				t.Errorf("RunePointer(%v) = %v, want %v", tt.in, *got, tt.want)
			}
		})
	}
}

// TestRunesPointer_Table verifies RunesPointer always returns a non-nil pointer with the converted slice.
func TestRunesPointer_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []int32
	}{
		{name: "nil", in: nil, want: nil},
		{name: "string slice", in: []string{"65", "66"}, want: []int32{65, 66}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.RunesPointer(tt.in)
			if got == nil {
				t.Fatalf("RunesPointer(%v) = nil, want non-nil pointer", tt.in)
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("RunesPointer(%v) = %v, want %v", tt.in, *got, tt.want)
			}
		})
	}
}

// TestDuration_Table verifies Duration for numeric, unit string, day suffix and invalid inputs.
func TestDuration_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want time.Duration
	}{
		{name: "nil", in: nil, want: 0},
		{name: "int nanoseconds", in: 1000, want: time.Duration(1000)},
		{name: "int64 nanoseconds", in: int64(5), want: time.Duration(5)},
		{name: "float nanoseconds", in: 2.9, want: time.Duration(2)},
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
		{name: "zero seconds", in: "0s", want: 0},
		{name: "invalid string", in: "abc", want: 0},
		{name: "bare number", in: "1000", want: 0},
		{name: "bool", in: true, want: 0},
		{name: "bytes", in: []byte("2s"), want: 2 * time.Second},
		{name: "duration", in: time.Duration(3), want: 3},
		{name: "string pointer", in: ptrTo("1s"), want: time.Second},
		{name: "nil string pointer", in: (*string)(nil), want: 0},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Duration(tt.in); got != tt.want {
				t.Errorf("Duration(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestDurations_Table verifies Durations for supported slices and unsupported inputs.
func TestDurations_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []time.Duration
	}{
		{name: "nil", in: nil, want: nil},
		{name: "string slice", in: []string{"1s", "2m"}, want: []time.Duration{time.Second, 2 * time.Minute}},
		{name: "int slice", in: []int{1, 2}, want: []time.Duration{time.Nanosecond, 2 * time.Nanosecond}},
		{name: "any slice", in: []any{"1s", 1000}, want: []time.Duration{time.Second, time.Duration(1000)}},
		{name: "nil string slice pointer", in: (*[]string)(nil), want: nil},
		{name: "unsupported", in: 123, want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Durations(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Durations(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestDurationPointer_Table verifies DurationPointer always returns a non-nil pointer with the converted value.
func TestDurationPointer_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want time.Duration
	}{
		{name: "nil", in: nil, want: 0},
		{name: "string", in: "1s", want: time.Second},
		{name: "int", in: 10, want: time.Duration(10)},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.DurationPointer(tt.in)
			if got == nil {
				t.Fatalf("DurationPointer(%v) = nil, want non-nil pointer", tt.in)
			}
			if *got != tt.want {
				t.Errorf("DurationPointer(%v) = %v, want %v", tt.in, *got, tt.want)
			}
		})
	}
}

// TestDurationsPointer_Table verifies DurationsPointer always returns a non-nil pointer with the converted slice.
func TestDurationsPointer_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []time.Duration
	}{
		{name: "nil", in: nil, want: nil},
		{name: "string slice", in: []string{"1h"}, want: []time.Duration{time.Hour}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.DurationsPointer(tt.in)
			if got == nil {
				t.Fatalf("DurationsPointer(%v) = nil, want non-nil pointer", tt.in)
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("DurationsPointer(%v) = %v, want %v", tt.in, *got, tt.want)
			}
		})
	}
}

// TestB_Table verifies B for capacity units, decimals and invalid inputs.
func TestB_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want float64
	}{
		{name: "nil", in: nil, want: 0},
		{name: "int", in: 1024, want: 1024},
		{name: "int64", in: int64(2048), want: 2048},
		{name: "bare number string", in: "123", want: 123},
		{name: "decimal without unit", in: "1.5", want: 1.5},
		{name: "bytes", in: "0B", want: 0},
		{name: "kilobyte", in: "1KB", want: float64(1 << 10)},
		{name: "lowercase kilobyte", in: "1kb", want: float64(1 << 10)},
		{name: "kilo", in: "1K", want: float64(1 << 10)},
		{name: "megabyte", in: "1MB", want: float64(1 << 20)},
		{name: "gigabyte", in: "1GB", want: float64(1 << 30)},
		{name: "terabyte", in: "1TB", want: float64(1 << 40)},
		{name: "petabyte", in: "1PB", want: float64(1 << 50)},
		{name: "exabyte", in: "1EB", want: float64(1 << 60)},
		{name: "zettabyte", in: "1ZB", want: float64(1 << 70)},
		{name: "decimal gigabyte", in: "1.5GB", want: 1.5 * float64(1<<30)},
		{name: "terabytes", in: "44TB", want: 44 * float64(1<<40)},
		{name: "invalid unit", in: "AM", want: 0},
		{name: "negative", in: "-44MB", want: 0},
		{name: "invalid string", in: "bad", want: 0},
		{name: "bool", in: true, want: 0},
		{name: "bytes input", in: []byte("2KB"), want: float64(2 << 10)},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.B(tt.in); got != tt.want {
				t.Errorf("B(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestBs_Table verifies Bs for supported slices and unsupported inputs.
func TestBs_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []float64
	}{
		{name: "nil", in: nil, want: nil},
		{name: "string slice", in: []string{"1KB", "1MB"}, want: []float64{float64(1 << 10), float64(1 << 20)}},
		{name: "int slice", in: []int{1, 2}, want: []float64{1, 2}},
		{name: "float slice", in: []float64{1.5, 2.5}, want: []float64{1.5, 2.5}},
		{name: "bool slice", in: []bool{true, false}, want: []float64{0, 0}},
		{name: "nil string slice pointer", in: (*[]string)(nil), want: nil},
		{name: "unsupported", in: 123, want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Bs(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Bs(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestJson_Table verifies Json for raw JSON, marshalled containers and unsupported inputs.
func TestJson_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "object string", in: `{"a":1}`, want: `{"a":1}`},
		{name: "array string", in: `[1,2]`, want: `[1,2]`},
		{name: "empty object", in: "{}", want: "{}"},
		{name: "empty array", in: "[]", want: "[]"},
		{name: "non json string", in: "abc", want: ""},
		{name: "number", in: 123, want: ""},
		{name: "bool", in: true, want: ""},
		{name: "json bytes", in: []byte(`{"a":1}`), want: `{"a":1}`},
		{name: "non json bytes", in: []byte("plain"), want: ""},
		{name: "nil string pointer", in: (*string)(nil), want: ""},
		{name: "nil bytes pointer", in: (*[]byte)(nil), want: ""},
		{name: "json string pointer", in: ptrTo(`{"k":"v"}`), want: `{"k":"v"}`},
		{name: "struct", in: struct {
			ID int `json:"id"`
		}{ID: 1}, want: `{"id":1}`},
		{name: "map", in: map[string]int{"a": 1}, want: `{"a":1}`},
		{name: "slice", in: []int{1, 2}, want: "[1,2]"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Json(tt.in); got != tt.want {
				t.Errorf("Json(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestAnys_Table verifies Anys for supported slices and arrays plus unsupported inputs.
func TestAnys_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []any
	}{
		{name: "nil", in: nil, want: nil},
		{name: "int slice", in: []int{1, 2, 3}, want: []any{1, 2, 3}},
		{name: "array", in: [3]string{"a", "b", "c"}, want: []any{"a", "b", "c"}},
		{name: "any slice", in: []any{1, "x", nil}, want: []any{1, "x", nil}},
		{name: "int slice pointer", in: ptrTo([]int{4, 5}), want: []any{4, 5}},
		{name: "unsupported int", in: 123, want: nil},
		{name: "unsupported string", in: "abc", want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := xconv.Anys(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Anys(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestAnysPointer_Table verifies AnysPointer always returns a non-nil pointer with the converted slice.
func TestAnysPointer_Table(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []any
	}{
		{name: "nil", in: nil, want: nil},
		{name: "int slice", in: []int{1, 2}, want: []any{1, 2}},
		{name: "unsupported", in: 123, want: nil},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.AnysPointer(tt.in)
			if got == nil {
				t.Fatalf("AnysPointer(%v) = nil, want non-nil pointer", tt.in)
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("AnysPointer(%v) = %v, want %v", tt.in, *got, tt.want)
			}
		})
	}
}

// runGenericNumbersTable runs the given cases against GenericNumbers[T].
func runGenericNumbersTable[T any](t *testing.T, cases []struct {
	name string
	in   any
	want []T
}) {
	t.Helper()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := xconv.GenericNumbers[T](tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenericNumbers[%s](%v) = %v, want %v",
					reflect.TypeOf((*T)(nil)).Elem(), tt.in, got, tt.want)
			}
		})
	}
}

// TestGenericNumbers_Table verifies GenericNumbers for integer, float, named and unsupported target types.
func TestGenericNumbers_Table(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		runGenericNumbersTable(t, []struct {
			name string
			in   any
			want []int
		}{
			{name: "nil", in: nil, want: nil},
			{name: "int slice", in: []int{1, 2, 3}, want: []int{1, 2, 3}},
			{name: "string slice", in: []string{"1", "2"}, want: []int{1, 2}},
			{name: "any slice", in: []any{1, "x", true}, want: []int{1, 0, 1}},
			{name: "array", in: [2]int{3, 4}, want: []int{3, 4}},
			{name: "unsupported", in: 123, want: nil},
		})
	})

	t.Run("float64", func(t *testing.T) {
		runGenericNumbersTable(t, []struct {
			name string
			in   any
			want []float64
		}{
			{name: "nil", in: nil, want: nil},
			{name: "string slice", in: []string{"1.5", "2"}, want: []float64{1.5, 2}},
			{name: "int slice", in: []int{1, 2}, want: []float64{1, 2}},
		})
	})

	t.Run("named int", func(t *testing.T) {
		runGenericNumbersTable(t, []struct {
			name string
			in   any
			want []miscInt
		}{
			{name: "int slice", in: []int{1, 2, 3}, want: []miscInt{1, 2, 3}},
		})
	})

	t.Run("unsupported element type", func(t *testing.T) {
		runGenericNumbersTable(t, []struct {
			name string
			in   any
			want []string
		}{
			{name: "int slice", in: []int{1, 2}, want: []string{"", ""}},
		})
	})
}

// TestUnsafe_Table verifies the zero-copy conversion of StringToBytes and BytesToString.
func TestUnsafe_Table(t *testing.T) {
	t.Run("StringToBytes", func(t *testing.T) {
		cases := []struct {
			name string
			in   string
		}{
			{name: "empty", in: ""},
			{name: "ascii", in: "hello"},
			{name: "unicode", in: "世界"},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				got := xconv.StringToBytes(tt.in)
				if string(got) != tt.in {
					t.Errorf("StringToBytes(%q) = %q, want %q", tt.in, got, tt.in)
				}
				if len(got) != len(tt.in) {
					t.Errorf("StringToBytes(%q) length = %d, want %d", tt.in, len(got), len(tt.in))
				}
				if len(tt.in) > 0 && unsafe.StringData(tt.in) != unsafe.SliceData(got) {
					t.Errorf("StringToBytes(%q) does not share the backing memory of the input", tt.in)
				}
			})
		}
	})

	t.Run("BytesToString", func(t *testing.T) {
		cases := []struct {
			name string
			in   []byte
			want string
		}{
			{name: "empty", in: []byte{}, want: ""},
			{name: "ascii", in: []byte("hello"), want: "hello"},
			{name: "unicode", in: []byte("世界"), want: "世界"},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				got := xconv.BytesToString(tt.in)
				if got != tt.want {
					t.Errorf("BytesToString(%v) = %q, want %q", tt.in, got, tt.want)
				}
				if len(got) != len(tt.in) {
					t.Errorf("BytesToString(%v) length = %d, want %d", tt.in, len(got), len(tt.in))
				}
				if len(tt.in) > 0 && unsafe.StringData(got) != unsafe.SliceData(tt.in) {
					t.Errorf("BytesToString(%v) does not share the backing memory of the input", tt.in)
				}
			})
		}
	})
}
