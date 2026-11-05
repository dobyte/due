package net_test

import (
	stdnet "net"
	"testing"

	"github.com/dobyte/due/v2/core/net"
)

const (
	stubPublicIP  = "203.0.113.10"
	stubPrivateIP = "192.168.1.10"
)

// stubResolvers replaces the global IP resolvers with deterministic ones, so that the tests do not
// depend on external network services.
func stubResolvers() {
	net.SetPublicIPResolver(func() (string, error) { return stubPublicIP, nil })
	net.SetPrivateIPResolver(func() (string, error) { return stubPrivateIP, nil })
}

// requireIPv6 skips the test when the system has no usable IPv6 loopback.
func requireIPv6(t *testing.T) {
	t.Helper()

	probe, err := stdnet.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("IPv6 is not available")
	}

	_ = probe.Close()
}

// splitHostPort splits an address into host and port, failing the test when the address is invalid.
func splitHostPort(t *testing.T, addr string) (string, string) {
	t.Helper()

	host, port, err := stdnet.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port failed, addr: %s, err: %v", addr, err)
	}

	return host, port
}

func TestParseAddr(t *testing.T) {
	stubResolvers()

	t.Run("wildcard addr with public expose", func(t *testing.T) {
		listenAddr, exposeAddr, err := net.ParseAddr("0.0.0.0:0", true)
		if err != nil {
			t.Fatalf("parse addr failed: %v", err)
		}

		listenHost, listenPort := splitHostPort(t, listenAddr)
		if listenHost != net.IPv4Zero {
			t.Fatalf("invalid listen host, expect: %s, actual: %s", net.IPv4Zero, listenHost)
		}
		if listenPort == "" || listenPort == "0" {
			t.Fatalf("invalid listen port, actual: %s", listenPort)
		}

		exposeHost, exposePort := splitHostPort(t, exposeAddr)
		if exposeHost != stubPublicIP {
			t.Fatalf("invalid expose host, expect: %s, actual: %s", stubPublicIP, exposeHost)
		}
		if exposePort != listenPort {
			t.Fatalf("expose port mismatched, expect: %s, actual: %s", listenPort, exposePort)
		}
	})

	t.Run("wildcard addr with private expose", func(t *testing.T) {
		listenAddr, exposeAddr, err := net.ParseAddr(":3553")
		if err != nil {
			t.Fatalf("parse addr failed: %v", err)
		}

		if listenAddr != ":3553" {
			t.Fatalf("invalid listen addr, actual: %s", listenAddr)
		}
		if exposeAddr != stdnet.JoinHostPort(stubPrivateIP, "3553") {
			t.Fatalf("invalid expose addr, actual: %s", exposeAddr)
		}
	})

	t.Run("explicit host", func(t *testing.T) {
		listenAddr, exposeAddr, err := net.ParseAddr("127.0.0.1:3553")
		if err != nil {
			t.Fatalf("parse addr failed: %v", err)
		}

		if listenAddr != "127.0.0.1:3553" || exposeAddr != "127.0.0.1:3553" {
			t.Fatalf("invalid addr, listen: %s, expose: %s", listenAddr, exposeAddr)
		}
	})

	t.Run("explicit host with random port", func(t *testing.T) {
		listenAddr, exposeAddr, err := net.ParseAddr("127.0.0.1:0")
		if err != nil {
			t.Fatalf("parse addr failed: %v", err)
		}

		listenHost, listenPort := splitHostPort(t, listenAddr)
		if listenHost != "127.0.0.1" {
			t.Fatalf("invalid listen host, actual: %s", listenHost)
		}
		if listenPort == "" || listenPort == "0" {
			t.Fatalf("invalid listen port, actual: %s", listenPort)
		}
		if exposeAddr != listenAddr {
			t.Fatalf("expose addr mismatched, expect: %s, actual: %s", listenAddr, exposeAddr)
		}
	})

	t.Run("ipv6 wildcard addr with fixed port", func(t *testing.T) {
		listenAddr, exposeAddr, err := net.ParseAddr("[::]:3553")
		if err != nil {
			t.Fatalf("parse addr failed: %v", err)
		}

		if listenAddr != stdnet.JoinHostPort(net.IPv6Zero, "3553") {
			t.Fatalf("invalid listen addr, actual: %s", listenAddr)
		}
		if exposeAddr != stdnet.JoinHostPort(stubPrivateIP, "3553") {
			t.Fatalf("invalid expose addr, actual: %s", exposeAddr)
		}
	})

	t.Run("ipv6 wildcard addr with random port", func(t *testing.T) {
		requireIPv6(t)

		listenAddr, exposeAddr, err := net.ParseAddr("[::]:0")
		if err != nil {
			t.Fatalf("parse addr failed: %v", err)
		}

		listenHost, listenPort := splitHostPort(t, listenAddr)
		if listenHost != net.IPv6Zero {
			t.Fatalf("invalid listen host, expect: %s, actual: %s", net.IPv6Zero, listenHost)
		}
		if listenPort == "" || listenPort == "0" {
			t.Fatalf("invalid listen port, actual: %s", listenPort)
		}
		if exposeAddr != stdnet.JoinHostPort(stubPrivateIP, listenPort) {
			t.Fatalf("invalid expose addr, actual: %s", exposeAddr)
		}
	})

	t.Run("ipv6 host with random port", func(t *testing.T) {
		requireIPv6(t)

		listenAddr, exposeAddr, err := net.ParseAddr("[::1]:0")
		if err != nil {
			t.Fatalf("parse addr failed: %v", err)
		}

		listenHost, listenPort := splitHostPort(t, listenAddr)
		if listenHost != "::1" {
			t.Fatalf("invalid listen host, expect: ::1, actual: %s", listenHost)
		}
		if listenPort == "" || listenPort == "0" {
			t.Fatalf("invalid listen port, actual: %s", listenPort)
		}
		if exposeAddr != listenAddr {
			t.Fatalf("expose addr mismatched, expect: %s, actual: %s", listenAddr, exposeAddr)
		}
	})

	t.Run("invalid addr", func(t *testing.T) {
		if _, _, err := net.ParseAddr("127.0.0.1"); err == nil {
			t.Fatal("expect an error for the invalid address")
		}
	})
}

func TestListenNetwork(t *testing.T) {
	tests := []struct {
		name    string
		network string
		addr    string
		want    string
	}{
		{name: "empty host", network: "tcp", addr: ":3553", want: "tcp"},
		{name: "ipv6 wildcard", network: "tcp", addr: "[::]:3553", want: "tcp"},
		{name: "ipv4 wildcard", network: "tcp", addr: "0.0.0.0:3553", want: "tcp4"},
		{name: "ipv4 host", network: "tcp", addr: "127.0.0.1:3553", want: "tcp4"},
		{name: "ipv6 host", network: "tcp", addr: "[::1]:3553", want: "tcp"},
		{name: "hostname", network: "tcp", addr: "localhost:3553", want: "tcp"},
		{name: "udp ipv4 wildcard", network: "udp", addr: "0.0.0.0:3553", want: "udp4"},
		{name: "udp ipv6 wildcard", network: "udp", addr: "[::]:3553", want: "udp"},
		{name: "invalid addr", network: "tcp", addr: "invalid", want: "tcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := net.ListenNetwork(tt.network, tt.addr); got != tt.want {
				t.Fatalf("invalid network, expect: %s, actual: %s", tt.want, got)
			}
		})
	}
}

func TestFulfillAddr(t *testing.T) {
	if addr := net.FulfillAddr(":3553"); addr != "[::]:3553" {
		t.Fatalf("invalid addr, expect: [::]:3553, actual: %s", addr)
	}

	if addr := net.FulfillAddr("[::]:3553"); addr != "[::]:3553" {
		t.Fatalf("invalid addr, expect: [::]:3553, actual: %s", addr)
	}

	if addr := net.FulfillAddr("0.0.0.0:3553"); addr != "0.0.0.0:3553" {
		t.Fatalf("invalid addr, expect: 0.0.0.0:3553, actual: %s", addr)
	}

	if addr := net.FulfillAddr("127.0.0.1:3553"); addr != "127.0.0.1:3553" {
		t.Fatalf("invalid addr, expect: 127.0.0.1:3553, actual: %s", addr)
	}

	if addr := net.FulfillAddr("invalid"); addr != "invalid" {
		t.Fatalf("invalid addr, expect: invalid, actual: %s", addr)
	}
}

func TestParseHostPort(t *testing.T) {
	host, port, err := net.ParseHostPort("tcp://127.0.0.1:3553")
	if err != nil {
		t.Fatalf("parse host port failed: %v", err)
	}
	if host != "127.0.0.1" || port != 3553 {
		t.Fatalf("invalid host port, host: %s, port: %d", host, port)
	}

	if _, _, err = net.ParseHostPort("tcp://127.0.0.1"); err == nil {
		t.Fatal("expect an error for the endpoint without a port")
	}

	if _, _, err = net.ParseHostPort("tcp://127.0.0.1:port"); err == nil {
		t.Fatal("expect an error for the endpoint with an invalid port")
	}
}

func TestExtractIPAndPort(t *testing.T) {
	addr := &stdnet.TCPAddr{IP: stdnet.ParseIP("127.0.0.1"), Port: 3553}

	ip, err := net.ExtractIP(addr)
	if err != nil {
		t.Fatalf("extract ip failed: %v", err)
	}
	if ip != "127.0.0.1" {
		t.Fatalf("invalid ip, expect: 127.0.0.1, actual: %s", ip)
	}

	port, err := net.ExtractPort(addr)
	if err != nil {
		t.Fatalf("extract port failed: %v", err)
	}
	if port != 3553 {
		t.Fatalf("invalid port, expect: 3553, actual: %d", port)
	}
}

func TestAssignRandPort(t *testing.T) {
	port, err := net.AssignRandPort()
	if err != nil {
		t.Fatalf("assign random port failed: %v", err)
	}
	if port <= 0 {
		t.Fatalf("invalid port, actual: %d", port)
	}

	port, err = net.AssignRandPort("127.0.0.1")
	if err != nil {
		t.Fatalf("assign random port failed: %v", err)
	}
	if port <= 0 {
		t.Fatalf("invalid port, actual: %d", port)
	}

	requireIPv6(t)

	port, err = net.AssignRandPort("::1")
	if err != nil {
		t.Fatalf("assign random port failed: %v", err)
	}
	if port <= 0 {
		t.Fatalf("invalid port, actual: %d", port)
	}
}

func TestPublicIPAndPrivateIP(t *testing.T) {
	stubResolvers()

	ip, err := net.PublicIP()
	if err != nil {
		t.Fatalf("get public ip failed: %v", err)
	}
	if ip != stubPublicIP {
		t.Fatalf("invalid public ip, expect: %s, actual: %s", stubPublicIP, ip)
	}

	ip, err = net.PrivateIP()
	if err != nil {
		t.Fatalf("get private ip failed: %v", err)
	}
	if ip != stubPrivateIP {
		t.Fatalf("invalid private ip, expect: %s, actual: %s", stubPrivateIP, ip)
	}
}

// TestDeprecatedIPFuncs ensures the deprecated helpers still delegate to PublicIP and PrivateIP.
func TestDeprecatedIPFuncs(t *testing.T) {
	stubResolvers()

	ip, err := net.ExternalIP()
	if err != nil {
		t.Fatalf("get external ip failed: %v", err)
	}
	if ip != stubPublicIP {
		t.Fatalf("invalid external ip, expect: %s, actual: %s", stubPublicIP, ip)
	}

	ip, err = net.InternalIP()
	if err != nil {
		t.Fatalf("get internal ip failed: %v", err)
	}
	if ip != stubPrivateIP {
		t.Fatalf("invalid internal ip, expect: %s, actual: %s", stubPrivateIP, ip)
	}
}

// TestExtractIPAndPortSpecialCases covers the address forms unwrapped without splitting the string
// form: a zoned IPv6 address and an address without an IP.
func TestExtractIPAndPortSpecialCases(t *testing.T) {
	zoned := &stdnet.TCPAddr{IP: stdnet.ParseIP("fe80::1"), Port: 3553, Zone: "eth0"}

	if ip, err := net.ExtractIP(zoned); err != nil || ip != "fe80::1%eth0" {
		t.Fatalf("ExtractIP(zoned) = (%q, %v), want (fe80::1%%eth0, nil)", ip, err)
	}
	if port, err := net.ExtractPort(zoned); err != nil || port != 3553 {
		t.Fatalf("ExtractPort(zoned) = (%d, %v), want (3553, nil)", port, err)
	}

	empty := &stdnet.UDPAddr{Port: 3553}

	if ip, err := net.ExtractIP(empty); err != nil || ip != "" {
		t.Fatalf("ExtractIP(empty) = (%q, %v), want (\"\", nil)", ip, err)
	}
	if port, err := net.ExtractPort(empty); err != nil || port != 3553 {
		t.Fatalf("ExtractPort(empty) = (%d, %v), want (3553, nil)", port, err)
	}
}

func BenchmarkExtractIP(b *testing.B) {
	tests := []struct {
		name string
		addr stdnet.Addr
	}{
		{name: "tcp ipv4", addr: &stdnet.TCPAddr{IP: stdnet.ParseIP("192.168.0.88"), Port: 3553}},
		{name: "udp ipv6", addr: &stdnet.UDPAddr{IP: stdnet.ParseIP("2408:db8::1"), Port: 3553}},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, _ = net.ExtractIP(tt.addr)
			}
		})
	}
}

func BenchmarkExtractPort(b *testing.B) {
	tests := []struct {
		name string
		addr stdnet.Addr
	}{
		{name: "tcp", addr: &stdnet.TCPAddr{IP: stdnet.ParseIP("192.168.0.88"), Port: 3553}},
		{name: "udp", addr: &stdnet.UDPAddr{IP: stdnet.ParseIP("2408:db8::1"), Port: 3553}},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, _ = net.ExtractPort(tt.addr)
			}
		})
	}
}
