package internal_test

import (
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/log/internal"
)

// TestString_Values verifies the string representation of the non-pointer scalar types.
func TestString_Values(t *testing.T) {
	tm := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "int", in: int(-1), want: "-1"},
		{name: "int8", in: int8(-2), want: "-2"},
		{name: "int16", in: int16(-3), want: "-3"},
		{name: "int32", in: int32(-4), want: "-4"},
		{name: "int64", in: int64(-5), want: "-5"},
		{name: "uint", in: uint(1), want: "1"},
		{name: "uint8", in: uint8(2), want: "2"},
		{name: "uint16", in: uint16(3), want: "3"},
		{name: "uint32", in: uint32(4), want: "4"},
		{name: "uint64", in: uint64(5), want: "5"},
		{name: "float32", in: float32(1.5), want: "1.5"},
		{name: "float64", in: float64(2.5), want: "2.5"},
		{name: "complex64", in: complex64(1 + 2i), want: strconv.FormatComplex(complex128(complex64(1+2i)), 'e', -1, 64)},
		{name: "complex128", in: complex128(3 + 4i), want: strconv.FormatComplex(complex128(3+4i), 'e', -1, 128)},
		{name: "bool", in: true, want: "true"},
		{name: "string", in: "hello", want: "hello"},
		{name: "time", in: tm, want: tm.String()},
		{name: "default", in: []byte{1, 2}, want: "[1 2]"},
		{name: "nil", in: nil, want: "<nil>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := internal.String(tt.in); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestString_Pointers verifies that pointer values are rendered as hexadecimal addresses.
func TestString_Pointers(t *testing.T) {
	i := 1
	i8 := int8(1)
	i16 := int16(1)
	i32 := int32(1)
	i64 := int64(1)
	u := uint(1)
	u8 := uint8(1)
	u16 := uint16(1)
	u32 := uint32(1)
	u64 := uint64(1)
	f32 := float32(1)
	f64 := float64(1)
	c64 := complex64(1 + 2i)
	c128 := complex128(3 + 4i)
	b := true
	s := "x"
	tm := time.Now()

	pointers := []any{
		&i, &i8, &i16, &i32, &i64,
		&u, &u8, &u16, &u32, &u64,
		&f32, &f64, &c64, &c128, &b, &s,
	}

	for _, v := range pointers {
		if got := internal.String(v); !strings.HasPrefix(got, "0x") {
			t.Errorf("String(%T) = %q, want a 0x-prefixed address", v, got)
		}
	}

	if got, want := internal.String(&tm), tm.String(); got != want {
		t.Errorf("String(*time.Time) = %q, want %q", got, want)
	}
}

// TestLevel_Methods verifies the level priority, color and label mappings.
func TestLevel_Methods(t *testing.T) {
	tests := []struct {
		level    internal.Level
		priority int
		color    string
		label    string
	}{
		{level: internal.LevelNone, priority: 0, color: "\x1b[36m", label: "NONE"},
		{level: internal.LevelDebug, priority: 1, color: "\x1b[37m", label: "DEBU"},
		{level: internal.LevelInfo, priority: 2, color: "\x1b[36m", label: "INFO"},
		{level: internal.LevelWarn, priority: 3, color: "\x1b[33m", label: "WARN"},
		{level: internal.LevelError, priority: 4, color: "\x1b[31m", label: "ERRO"},
		{level: internal.LevelFatal, priority: 5, color: "\x1b[31m", label: "FATA"},
		{level: internal.LevelPanic, priority: 6, color: "\x1b[31m", label: "PANI"},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			if got := tt.level.Priority(); got != tt.priority {
				t.Errorf("Priority() = %d, want %d", got, tt.priority)
			}
			if got := tt.level.Color(); got != tt.color {
				t.Errorf("Color() = %q, want %q", got, tt.color)
			}
			if got := tt.level.Label(); got != tt.label {
				t.Errorf("Label() = %q, want %q", got, tt.label)
			}
		})
	}

	var unknown internal.Level = "unknown"
	if got := unknown.Label(); got != "NONE" {
		t.Errorf("unknown Label() = %q, want %q", got, "NONE")
	}
}

// TestTextFormatter verifies the text formatter with and without colors and stacks.
func TestTextFormatter(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		f := internal.NewTextFormatter()
		buf := f.Format(&internal.Entity{
			Time:    "2024/01/02 03:04:05.000000",
			Level:   internal.LevelInfo,
			Message: "hello",
			Caller:  "main.go:10",
		})
		defer buf.Release()

		got := string(buf.Bytes())
		for _, want := range []string{"INFO", "2024/01/02", "main.go:10", "hello"} {
			if !strings.Contains(got, want) {
				t.Errorf("formatted = %q, want it to contain %q", got, want)
			}
		}
		if strings.Contains(got, "\x1b") {
			t.Errorf("formatted = %q, want no color codes", got)
		}
	})

	t.Run("colored with stack", func(t *testing.T) {
		f := internal.NewTextFormatter(true)
		buf := f.Format(&internal.Entity{
			Time:   "2024/01/02 03:04:05.000000",
			Level:  internal.LevelError,
			Frames: []runtime.Frame{{Function: "main.run", File: "main.go", Line: 42}},
		})
		defer buf.Release()

		got := string(buf.Bytes())
		for _, want := range []string{"\x1b[31m", "ERRO", "Stack:", "main.run"} {
			if !strings.Contains(got, want) {
				t.Errorf("formatted = %q, want it to contain %q", got, want)
			}
		}
	})
}

// TestJsonFormatter verifies the JSON formatter and its string escaping.
func TestJsonFormatter(t *testing.T) {
	f := internal.NewJsonFormatter()
	buf := f.Format(&internal.Entity{
		Time:    "2024/01/02 03:04:05.000000",
		Level:   internal.LevelWarn,
		Caller:  "main.go:10",
		Message: "quote \" backslash \\ newline \n tab \t angle <>\x01",
		Frames:  []runtime.Frame{{Function: "main.run", File: "main.go", Line: 42}},
	})
	defer buf.Release()

	got := string(buf.Bytes())
	for _, want := range []string{`\"`, `\\`, `\n`, `\t`, `\u003c`, `\u003e`, `\u0001`, `"stack"`} {
		if !strings.Contains(got, want) {
			t.Errorf("formatted = %q, want it to contain %q", got, want)
		}
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("formatted output is not valid JSON: %v\n%s", err, got)
	}
	if decoded["level"] != "WARN" {
		t.Errorf("level = %v, want WARN", decoded["level"])
	}
	if decoded["msg"] != "quote \" backslash \\ newline \n tab \t angle <>\x01" {
		t.Errorf("msg = %q, want the original message", decoded["msg"])
	}
}

// TestJsonFormatter_Minimal verifies the JSON formatter without caller, message and stack.
func TestJsonFormatter_Minimal(t *testing.T) {
	f := internal.NewJsonFormatter()
	buf := f.Format(&internal.Entity{
		Time:  "2024/01/02 03:04:05.000000",
		Level: internal.LevelInfo,
	})
	defer buf.Release()

	got := string(buf.Bytes())
	if strings.Contains(got, `"msg"`) || strings.Contains(got, `"file"`) || strings.Contains(got, `"stack"`) {
		t.Errorf("formatted = %q, want no optional fields", got)
	}
}
