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

// TestIP2Long_ValidIPv6 verifies that valid IPv6 addresses round-trip correctly.
func TestIP2Long_ValidIPv6(t *testing.T) {
	cases := []string{
		"::",
		"::1",
		"2408:db8::1",
		"2001:db8::ff00:42:8329",
	}

	for _, ip := range cases {
		long := IP2Long(ip)
		if got := Long2IP(long); got != ip {
			t.Errorf("Long2IP(IP2Long(%q)) = %q, want %q", ip, got, ip)
		}
	}
}

// TestIP2Long_Invalid verifies that invalid addresses return the zero value.
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
