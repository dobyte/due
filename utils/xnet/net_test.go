package xnet

import (
	"encoding/binary"
	"testing"
)

// TestIP2Long_ValidIPv4 verifies that valid IPv4 addresses round-trip correctly.
func TestIP2Long_ValidIPv4(t *testing.T) {
	cases := []string{
		"0.0.0.0",
		"127.0.0.1",
		"218.108.212.34",
		"255.255.255.255",
	}

	for _, ip := range cases {
		if got := Long2IP(IP2Long(ip)); got != ip {
			t.Errorf("Long2IP(IP2Long(%q)) = %q, want %q", ip, got, ip)
		}
	}

	if got := IP2Long("218.108.212.34"); got != 0xDA6CD422 {
		t.Errorf("IP2Long(218.108.212.34) = %#x, want %#x", got, uint32(0xDA6CD422))
	}
}

// TestIP2Long_Invalid verifies that invalid and non-IPv4 addresses return 0.
func TestIP2Long_Invalid(t *testing.T) {
	cases := []string{
		"invalid",
		"256.1.1.1",
		"::1",
		"",
	}

	for _, ip := range cases {
		if got := IP2Long(ip); got != 0 {
			t.Errorf("IP2Long(%q) = %d, want 0", ip, got)
		}
	}
}

// TestIP2Bytes_Valid verifies that valid IPv4 and IPv6 addresses round-trip correctly.
func TestIP2Bytes_Valid(t *testing.T) {
	cases := []string{
		"0.0.0.0",
		"127.0.0.1",
		"218.108.212.34",
		"::",
		"::1",
		"2408:db8::1",
		"2001:db8::ff00:42:8329",
	}

	for _, ip := range cases {
		if got := Bytes2IP(IP2Bytes(ip)); got != ip {
			t.Errorf("Bytes2IP(IP2Bytes(%q)) = %q, want %q", ip, got, ip)
		}
	}

	raw := IP2Bytes("218.108.212.34")
	if got := binary.BigEndian.Uint32(raw[12:]); got != 0xDA6CD422 {
		t.Errorf("IP2Bytes(218.108.212.34) = %#x, want %#x", got, uint32(0xDA6CD422))
	}
}

// TestIP2Bytes_Invalid verifies that invalid addresses return the zero value.
func TestIP2Bytes_Invalid(t *testing.T) {
	cases := []string{
		"invalid",
		"256.1.1.1",
		"",
	}

	var zero [16]byte
	for _, ip := range cases {
		if got := IP2Bytes(ip); got != zero {
			t.Errorf("IP2Bytes(%q) = %v, want %v", ip, got, zero)
		}
	}
}

// TestIP2BytesZone verifies that a zoned IPv6 address keeps its address bytes.
func TestIP2BytesZone(t *testing.T) {
	if got, want := IP2Bytes("fe80::1%eth0"), IP2Bytes("fe80::1"); got != want {
		t.Errorf("IP2Bytes(fe80::1%%eth0) = %v, want %v", got, want)
	}
}

func BenchmarkIP2Long(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = IP2Long("218.108.212.34")
	}
}

func BenchmarkIP2Bytes(b *testing.B) {
	tests := []struct {
		name string
		ip   string
	}{
		{name: "ipv4", ip: "218.108.212.34"},
		{name: "ipv6", ip: "2408:db8::1"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = IP2Bytes(tt.ip)
			}
		})
	}
}
