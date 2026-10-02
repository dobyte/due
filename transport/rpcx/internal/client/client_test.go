package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	derrors "github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
)

// newCAFile generates a self-signed CA certificate file for TLS-related tests.
func newCAFile(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// failDiscovery simulates a registry initialization failure.
type failDiscovery struct{}

func (d *failDiscovery) Watch(_ context.Context, _ string) (registry.Watcher, error) {
	return nil, errors.New("discovery unavailable")
}

func (d *failDiscovery) Services(_ context.Context, _ string) ([]*registry.ServiceInstance, error) {
	return nil, errors.New("discovery unavailable")
}

// TestBuilderClose verifies that the Close path releases all connection pool resources.
func TestBuilderClose(t *testing.T) {
	b := NewBuilder(&Options{})
	if b.err != nil {
		t.Fatalf("NewBuilder error: %v", b.err)
	}

	cli, err := b.Build("direct://127.0.0.1:8011")
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	_ = cli

	// The connection pool is cached.
	if _, ok := b.pools.Load("direct://127.0.0.1:8011"); !ok {
		t.Fatal("pool should be cached")
	}

	if err := b.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	// The connection pools are cleared.
	var remain int
	b.pools.Range(func(_, _ any) bool {
		remain++
		return true
	})
	if remain != 0 {
		t.Errorf("pools should be cleared, %d remain", remain)
	}

	// Close is idempotent.
	if err := b.Close(); err != nil {
		t.Errorf("second Close should return nil, got %v", err)
	}

	// Build returns ErrClientClosed after Close.
	if _, err := b.Build("direct://127.0.0.1:8011"); !derrors.Is(err, derrors.ErrClientClosed) {
		t.Errorf("Build after Close should return ErrClientClosed, got %v", err)
	}
}

// TestBuilderTLSValidation verifies the TLS configuration behavior: TLS is enabled when both the
// certificate and the server name are configured, and a warning fallback to plaintext occurs when
// only one of them is set.
func TestBuilderTLSValidation(t *testing.T) {
	// Neither is configured -> plaintext connection.
	b := NewBuilder(&Options{})
	if b.err != nil {
		t.Fatalf("want nil error for plaintext, got %v", b.err)
	}
	if b.dialOpts.TLSConfig != nil {
		t.Fatal("TLSConfig should be nil for plaintext")
	}

	// Only ServerName is configured -> warning fallback to plaintext, no error.
	b = NewBuilder(&Options{ServerName: "example.com"})
	if b.err != nil {
		t.Fatalf("want nil error for warning fallback, got %v", b.err)
	}
	if b.dialOpts.TLSConfig != nil {
		t.Fatal("TLSConfig should be nil when only ServerName is set")
	}

	// Only CAFile is configured (the file does not exist) -> warning fallback to plaintext, no error.
	b = NewBuilder(&Options{CAFile: "not-exist.pem"})
	if b.err != nil {
		t.Fatalf("want nil error for warning fallback, got %v", b.err)
	}
	if b.dialOpts.TLSConfig != nil {
		t.Fatal("TLSConfig should be nil when only CAFile is set")
	}

	// Both CAFile and ServerName are configured -> TLS is enabled.
	caFile := newCAFile(t)
	b = NewBuilder(&Options{CAFile: caFile, ServerName: "localhost"})
	if b.err != nil {
		t.Fatalf("CAFile+ServerName should enable TLS, got err: %v", b.err)
	}
	if b.dialOpts.TLSConfig == nil {
		t.Fatal("TLSConfig should be set when both are configured")
	}
}

// TestBuilderInitError verifies that a registry initialization failure is returned through Build
// rather than being fatal.
func TestBuilderInitError(t *testing.T) {
	b := NewBuilder(&Options{Discovery: &failDiscovery{}})

	if _, err := b.Build("direct://127.0.0.1:8011"); err == nil {
		t.Fatal("Build should return the init error")
	}
}
