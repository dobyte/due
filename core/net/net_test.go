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

		if listenAddr != stdnet.JoinHostPort(net.IPv4Zero, "3553") {
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

	t.Run("invalid addr", func(t *testing.T) {
		if _, _, err := net.ParseAddr("127.0.0.1"); err == nil {
			t.Fatal("expect an error for the invalid address")
		}
	})
}

func TestFulfillAddr(t *testing.T) {
	if addr := net.FulfillAddr(":3553"); addr != "0.0.0.0:3553" {
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
