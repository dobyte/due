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
		long := IP2Long(ip)
		if got := Long2IP(long); got != ip {
			t.Errorf("Long2IP(IP2Long(%q)) = %q, want %q", ip, got, ip)
		}
	}

	long := IP2Long("218.108.212.34")
	if got := binary.BigEndian.Uint32(long[12:]); got != 0xDA6CD422 {
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
		"",
	}

	var zero [16]byte
	for _, ip := range cases {
		if got := IP2Long(ip); got != zero {
			t.Errorf("IP2Long(%q) = %v, want %v", ip, got, zero)
		}
	}
}
