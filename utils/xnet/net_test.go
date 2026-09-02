package xnet

import (
	"testing"
)

// TestIP2Long_ValidIPv4 验证合法的IPv4地址可正确往返转换
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

	if got := IP2Long("218.108.212.34"); got != 0xDA6CD422 {
		t.Errorf("IP2Long(218.108.212.34) = %d, want %d", got, 0xDA6CD422)
	}
}

// TestIP2Long_Invalid 验证非法及非IPv4地址返回0
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
