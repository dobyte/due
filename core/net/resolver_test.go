package net

import (
	stdnet "net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
)

// newIPServer starts a local HTTP server that always answers with the given body.
func newIPServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server
}

func TestDoQueryPublicIP(t *testing.T) {
	t.Run("valid ip", func(t *testing.T) {
		server := newIPServer(t, "203.0.113.7")

		ip, err := doQueryPublicIP(server.URL, time.Second)
		if err != nil {
			t.Fatalf("query public ip failed: %v", err)
		}
		if ip != "203.0.113.7" {
			t.Fatalf("invalid public ip, expect: 203.0.113.7, actual: %s", ip)
		}
	})

	t.Run("invalid ip", func(t *testing.T) {
		server := newIPServer(t, "not-an-ip")

		if _, err := doQueryPublicIP(server.URL, time.Second); !errors.Is(err, errors.ErrNotFoundIPAddress) {
			t.Fatalf("expect ErrNotFoundIPAddress, actual: %v", err)
		}
	})

	t.Run("unreachable endpoint", func(t *testing.T) {
		if _, err := doQueryPublicIP("http://127.0.0.1:1/ip", 50*time.Millisecond); err == nil {
			t.Fatal("expect an error for the unreachable endpoint")
		}
	})
}

// TestDefaultPublicIPResolver replaces the upstream URL list with a local server, so that the
// default resolver can be verified without reaching the public internet.
func TestDefaultPublicIPResolver(t *testing.T) {
	server := newIPServer(t, "198.51.100.9")

	original := urls
	urls = []string{server.URL}
	t.Cleanup(func() { urls = original })

	ip, err := defaultPublicIPResolver()
	if err != nil {
		t.Fatalf("resolve public ip failed: %v", err)
	}
	if ip != "198.51.100.9" {
		t.Fatalf("invalid public ip, expect: 198.51.100.9, actual: %s", ip)
	}
}

func TestDefaultPrivateIPResolver(t *testing.T) {
	ip, err := defaultPrivateIPResolver()
	if err != nil {
		if errors.Is(err, errors.ErrNotFoundIPAddress) {
			t.Skip("no private ip found on this host")
		}

		t.Fatalf("resolve private ip failed: %v", err)
	}

	if stdnet.ParseIP(ip) == nil {
		t.Fatalf("invalid private ip, actual: %s", ip)
	}
}
