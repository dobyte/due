package xhash_test

import (
	"testing"

	"github.com/dobyte/due/v2/utils/xhash"
)

func TestMD5(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "d41d8cd98f00b204e9800998ecf8427e"},
		{"abc", "abc", "900150983cd24fb0d6963f7d28e17f72"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xhash.MD5(tt.in); got != tt.want {
				t.Fatalf("MD5(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestSHA256(t *testing.T) {
	tests := []struct {
		name string
		data string
		key  string
		want string
	}{
		{
			name: "no key",
			data: "abc",
			key:  "",
			want: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name: "with key",
			data: "abc",
			key:  "key",
			want: "9c196e32dc0175f86f4b1cb89289d6619de6bee699e4c378e68309ed97a1a6ab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xhash.SHA256(tt.data, tt.key); got != tt.want {
				t.Fatalf("SHA256(%q, %q) = %s, want %s", tt.data, tt.key, got, tt.want)
			}
		})
	}
}

func TestFNV32(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want uint32
	}{
		{"empty", "", 2166136261},
		{"abc", "abc", 1134309195},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xhash.FNV32(tt.in); got != tt.want {
				t.Fatalf("FNV32(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestFNV32a(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want uint32
	}{
		{"empty", "", 2166136261},
		{"abc", "abc", 440920331},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xhash.FNV32a(tt.in); got != tt.want {
				t.Fatalf("FNV32a(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestFNV64(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want uint64
	}{
		{"empty", "", 14695981039346656037},
		{"abc", "abc", 15626587013303479755},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xhash.FNV64(tt.in); got != tt.want {
				t.Fatalf("FNV64(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestFNV64a(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want uint64
	}{
		{"empty", "", 14695981039346656037},
		{"abc", "abc", 16654208175385433931},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xhash.FNV64a(tt.in); got != tt.want {
				t.Fatalf("FNV64a(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
