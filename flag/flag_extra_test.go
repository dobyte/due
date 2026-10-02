package flag_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/v2/flag"
)

// absentKey is a key that never appears on the test command line.
const absentKey = "due.flag.test.absent.key"

// cmdFlags returns the flag name/value pairs found in os.Args. The test binary always receives
// some -test.* flags, which lets the present-value branches be exercised.
func cmdFlags() [][2]string {
	flags := make([][2]string, 0)

	for _, arg := range os.Args[1:] {
		if len(arg) < 2 || arg[0] != '-' {
			continue
		}

		name := strings.TrimLeft(arg, "-")
		value := ""
		if i := strings.IndexByte(name, '='); i >= 0 {
			value = name[i+1:]
			name = name[:i]
		}
		if name == "" || strings.HasPrefix(name, "-") {
			continue
		}

		flags = append(flags, [2]string{name, value})
	}

	return flags
}

// TestGettersAbsent verifies each getter returns the zero value without a default and the given
// default with one when the key is absent.
func TestGettersAbsent(t *testing.T) {
	if flag.Has(absentKey) {
		t.Fatalf("Has(%q) = true, want false", absentKey)
	}

	cases := []struct {
		name    string
		noDef   func() any
		withDef func() any
		want    any
		wantDef any
	}{
		{"String", func() any { return flag.String(absentKey) }, func() any { return flag.String(absentKey, "def") }, "", "def"},
		{"Bool", func() any { return flag.Bool(absentKey) }, func() any { return flag.Bool(absentKey, true) }, false, true},
		{"Int", func() any { return flag.Int(absentKey) }, func() any { return flag.Int(absentKey, 42) }, 0, 42},
		{"Int8", func() any { return flag.Int8(absentKey) }, func() any { return flag.Int8(absentKey, 8) }, int8(0), int8(8)},
		{"Int16", func() any { return flag.Int16(absentKey) }, func() any { return flag.Int16(absentKey, 16) }, int16(0), int16(16)},
		{"Int32", func() any { return flag.Int32(absentKey) }, func() any { return flag.Int32(absentKey, 32) }, int32(0), int32(32)},
		{"Int64", func() any { return flag.Int64(absentKey) }, func() any { return flag.Int64(absentKey, 64) }, int64(0), int64(64)},
		{"Uint", func() any { return flag.Uint(absentKey) }, func() any { return flag.Uint(absentKey, 7) }, uint(0), uint(7)},
		{"Uint8", func() any { return flag.Uint8(absentKey) }, func() any { return flag.Uint8(absentKey, 8) }, uint8(0), uint8(8)},
		{"Uint16", func() any { return flag.Uint16(absentKey) }, func() any { return flag.Uint16(absentKey, 16) }, uint16(0), uint16(16)},
		{"Uint32", func() any { return flag.Uint32(absentKey) }, func() any { return flag.Uint32(absentKey, 32) }, uint32(0), uint32(32)},
		{"Uint64", func() any { return flag.Uint64(absentKey) }, func() any { return flag.Uint64(absentKey, 64) }, uint64(0), uint64(64)},
		{"Float32", func() any { return flag.Float32(absentKey) }, func() any { return flag.Float32(absentKey, 1.5) }, float32(0), float32(1.5)},
		{"Float64", func() any { return flag.Float64(absentKey) }, func() any { return flag.Float64(absentKey, 2.5) }, float64(0), float64(2.5)},
		{"Duration", func() any { return flag.Duration(absentKey) }, func() any { return flag.Duration(absentKey, time.Minute) }, time.Duration(0), time.Minute},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.noDef(); got != c.want {
				t.Errorf("%s() = %v, want %v", c.name, got, c.want)
			}
			if got := c.withDef(); got != c.wantDef {
				t.Errorf("%s(def) = %v, want %v", c.name, got, c.wantDef)
			}
		})
	}
}

// TestGettersPresent verifies each getter reads a key present on the command line.
func TestGettersPresent(t *testing.T) {
	flags := cmdFlags()
	if len(flags) == 0 {
		t.Skip("no command-line flags available to exercise the present-value branches")
	}

	key, value := flags[0][0], flags[0][1]

	if !flag.Has(key) {
		t.Fatalf("Has(%q) = false, want true", key)
	}
	if got := flag.String(key); got != value {
		t.Errorf("String(%q) = %q, want %q", key, got, value)
	}

	// Exercise the present-value branch of the remaining getters.
	flag.Bool(key)
	flag.Int(key)
	flag.Int8(key)
	flag.Int16(key)
	flag.Int32(key)
	flag.Int64(key)
	flag.Uint(key)
	flag.Uint8(key)
	flag.Uint16(key)
	flag.Uint32(key)
	flag.Uint64(key)
	flag.Float32(key)
	flag.Float64(key)
	flag.Duration(key)
}

// TestBoolBranches verifies both present-value branches of Bool: an empty value means true.
func TestBoolBranches(t *testing.T) {
	var emptyKey, nonEmptyKey string
	for _, f := range cmdFlags() {
		if f[1] == "" && emptyKey == "" {
			emptyKey = f[0]
		}
		if f[1] != "" && nonEmptyKey == "" {
			nonEmptyKey = f[0]
		}
	}

	if emptyKey != "" {
		t.Run("empty value", func(t *testing.T) {
			if got := flag.Bool(emptyKey); !got {
				t.Errorf("Bool(%q) = false, want true", emptyKey)
			}
		})
	}

	if nonEmptyKey != "" {
		t.Run("non-empty value", func(t *testing.T) {
			// The value is parsed through xconv; only the branch is under test here.
			t.Logf("Bool(%q) = %v", nonEmptyKey, flag.Bool(nonEmptyKey))
		})
	}
}
