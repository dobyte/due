package xconv_test

import (
	"testing"
	"time"

	"github.com/dobyte/due/v2/utils/xconv"
)

// TestInt64_CorePointers covers every pointer branch of Int64.
func TestInt64_CorePointers(t *testing.T) {
	var (
		i    = 7
		i8   = int8(7)
		i16  = int16(7)
		i32  = int32(7)
		i64  = int64(7)
		u    = uint(7)
		u8   = uint8(7)
		u16  = uint16(7)
		u32  = uint32(7)
		u64  = uint64(7)
		f32  = float32(7)
		f64  = float64(7)
		c64  = complex64(7)
		c128 = complex128(7)
		b    = true
		tm   = time.Unix(0, 7)
		bs   = []byte{0, 0, 0, 0, 0, 0, 0, 7}
	)

	cases := []struct {
		name string
		in   any
		want int64
	}{
		{"*int", &i, 7},
		{"nil *int", (*int)(nil), 0},
		{"*int8", &i8, 7},
		{"nil *int8", (*int8)(nil), 0},
		{"*int16", &i16, 7},
		{"nil *int16", (*int16)(nil), 0},
		{"*int32", &i32, 7},
		{"nil *int32", (*int32)(nil), 0},
		{"*int64", &i64, 7},
		{"nil *int64", (*int64)(nil), 0},
		{"*uint", &u, 7},
		{"nil *uint", (*uint)(nil), 0},
		{"*uint8", &u8, 7},
		{"nil *uint8", (*uint8)(nil), 0},
		{"*uint16", &u16, 7},
		{"nil *uint16", (*uint16)(nil), 0},
		{"*uint32", &u32, 7},
		{"nil *uint32", (*uint32)(nil), 0},
		{"*uint64", &u64, 7},
		{"nil *uint64", (*uint64)(nil), 0},
		{"*float32", &f32, 7},
		{"nil *float32", (*float32)(nil), 0},
		{"*float64", &f64, 7},
		{"nil *float64", (*float64)(nil), 0},
		{"*complex64", &c64, 7},
		{"nil *complex64", (*complex64)(nil), 0},
		{"*complex128", &c128, 7},
		{"nil *complex128", (*complex128)(nil), 0},
		{"*bool", &b, 1},
		{"nil *bool", (*bool)(nil), 0},
		{"*time.Time", &tm, 7},
		{"nil *time.Time", (*time.Time)(nil), 0},
		{"*[]byte", &bs, 7},
		{"nil *[]byte", (*[]byte)(nil), 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := xconv.Int64(c.in); got != c.want {
				t.Errorf("Int64(%v) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

// TestUint64_CorePointers covers every pointer branch of Uint64.
func TestUint64_CorePointers(t *testing.T) {
	var (
		i    = 7
		i8   = int8(7)
		i16  = int16(7)
		i32  = int32(7)
		i64  = int64(7)
		u    = uint(7)
		u8   = uint8(7)
		u16  = uint16(7)
		u32  = uint32(7)
		u64  = uint64(7)
		f32  = float32(7)
		f64  = float64(7)
		c64  = complex64(7)
		c128 = complex128(7)
		b    = true
		tm   = time.Unix(0, 7)
		bs   = []byte{0, 0, 0, 0, 0, 0, 0, 7}
	)

	cases := []struct {
		name string
		in   any
		want uint64
	}{
		{"*int", &i, 7},
		{"nil *int", (*int)(nil), 0},
		{"*int8", &i8, 7},
		{"nil *int8", (*int8)(nil), 0},
		{"*int16", &i16, 7},
		{"nil *int16", (*int16)(nil), 0},
		{"*int32", &i32, 7},
		{"nil *int32", (*int32)(nil), 0},
		{"*int64", &i64, 7},
		{"nil *int64", (*int64)(nil), 0},
		{"*uint", &u, 7},
		{"nil *uint", (*uint)(nil), 0},
		{"*uint8", &u8, 7},
		{"nil *uint8", (*uint8)(nil), 0},
		{"*uint16", &u16, 7},
		{"nil *uint16", (*uint16)(nil), 0},
		{"*uint32", &u32, 7},
		{"nil *uint32", (*uint32)(nil), 0},
		{"*uint64", &u64, 7},
		{"nil *uint64", (*uint64)(nil), 0},
		{"*float32", &f32, 7},
		{"nil *float32", (*float32)(nil), 0},
		{"*float64", &f64, 7},
		{"nil *float64", (*float64)(nil), 0},
		{"*complex64", &c64, 7},
		{"nil *complex64", (*complex64)(nil), 0},
		{"*complex128", &c128, 7},
		{"nil *complex128", (*complex128)(nil), 0},
		{"*bool", &b, 1},
		{"nil *bool", (*bool)(nil), 0},
		{"*time.Time", &tm, 7},
		{"nil *time.Time", (*time.Time)(nil), 0},
		{"*[]byte", &bs, 7},
		{"nil *[]byte", (*[]byte)(nil), 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := xconv.Uint64(c.in); got != c.want {
				t.Errorf("Uint64(%v) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

// TestFloat64_CorePointers covers every pointer branch of Float64.
func TestFloat64_CorePointers(t *testing.T) {
	var (
		i    = 7
		i8   = int8(7)
		i16  = int16(7)
		i32  = int32(7)
		i64  = int64(7)
		u    = uint(7)
		u8   = uint8(7)
		u16  = uint16(7)
		u32  = uint32(7)
		u64  = uint64(7)
		f32  = float32(7)
		f64  = float64(7)
		c64  = complex64(7)
		c128 = complex128(7)
		b    = true
		tm   = time.Unix(0, 7)
		bs   = []byte{0, 0, 0, 0, 0, 0, 0, 7}
	)

	cases := []struct {
		name string
		in   any
		want float64
	}{
		{"*int", &i, 7},
		{"nil *int", (*int)(nil), 0},
		{"*int8", &i8, 7},
		{"nil *int8", (*int8)(nil), 0},
		{"*int16", &i16, 7},
		{"nil *int16", (*int16)(nil), 0},
		{"*int32", &i32, 7},
		{"nil *int32", (*int32)(nil), 0},
		{"*int64", &i64, 7},
		{"nil *int64", (*int64)(nil), 0},
		{"*uint", &u, 7},
		{"nil *uint", (*uint)(nil), 0},
		{"*uint8", &u8, 7},
		{"nil *uint8", (*uint8)(nil), 0},
		{"*uint16", &u16, 7},
		{"nil *uint16", (*uint16)(nil), 0},
		{"*uint32", &u32, 7},
		{"nil *uint32", (*uint32)(nil), 0},
		{"*uint64", &u64, 7},
		{"nil *uint64", (*uint64)(nil), 0},
		{"*float32", &f32, 7},
		{"nil *float32", (*float32)(nil), 0},
		{"*float64", &f64, 7},
		{"nil *float64", (*float64)(nil), 0},
		{"*complex64", &c64, 7},
		{"nil *complex64", (*complex64)(nil), 0},
		{"*complex128", &c128, 7},
		{"nil *complex128", (*complex128)(nil), 0},
		{"*bool", &b, 1},
		{"nil *bool", (*bool)(nil), 0},
		{"*time.Time", &tm, 7},
		{"nil *time.Time", (*time.Time)(nil), 0},
		{"*[]byte", &bs, 0},
		{"nil *[]byte", (*[]byte)(nil), 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := xconv.Float64(c.in); got != c.want {
				t.Errorf("Float64(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// TestJson_EdgeBranches covers the pointer, byte slice and reflected branches of Json.
func TestJson_EdgeBranches(t *testing.T) {
	obj := `{"a":1}`
	arr := "[1,2]"
	objBytes := []byte(obj)

	cases := []struct {
		name string
		in   any
		want string
	}{
		{"*string json", &obj, obj},
		{"*string plain", new(string), ""},
		{"nil *string", (*string)(nil), ""},
		{"[]byte json", []byte(arr), arr},
		{"*[]byte json", &objBytes, obj},
		{"nil *[]byte", (*[]byte)(nil), ""},
		{"jsonInt json", jsonInt(3), ""},
		{"jsonStr json", jsonStr(obj), obj},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := xconv.Json(c.in); got != c.want {
				t.Errorf("Json(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// jsonInt is a named integer type used to exercise the reflection fallback of Json.
type jsonInt int

// jsonStr is a named string type used to exercise the reflection fallback of Json.
type jsonStr string
