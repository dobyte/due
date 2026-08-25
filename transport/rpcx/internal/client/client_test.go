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

// newCAFile 生成一个自签名CA证书文件，用于TLS相关测试
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

// failDiscovery 模拟注册中心初始化失败的场景
type failDiscovery struct{}

func (d *failDiscovery) Watch(_ context.Context, _ string) (registry.Watcher, error) {
	return nil, errors.New("discovery unavailable")
}

func (d *failDiscovery) Services(_ context.Context, _ string) ([]*registry.ServiceInstance, error) {
	return nil, errors.New("discovery unavailable")
}

// TestBuilderClose 验证 Close 链路释放全部连接池资源
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

	// 连接池已缓存
	if _, ok := b.pools.Load("direct://127.0.0.1:8011"); !ok {
		t.Fatal("pool should be cached")
	}

	if err := b.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	// 连接池已清空
	var remain int
	b.pools.Range(func(_, _ any) bool {
		remain++
		return true
	})
	if remain != 0 {
		t.Errorf("pools should be cleared, %d remain", remain)
	}

	// Close 幂等
	if err := b.Close(); err != nil {
		t.Errorf("second Close should return nil, got %v", err)
	}

	// 关闭后 Build 返回 ErrClientClosed
	if _, err := b.Build("direct://127.0.0.1:8011"); !derrors.Is(err, derrors.ErrClientClosed) {
		t.Errorf("Build after Close should return ErrClientClosed, got %v", err)
	}
}

// TestBuilderTLSValidation 验证 TLS 配置行为：证书与校验域名均配置时启用TLS，单独配置时警告降级为明文
func TestBuilderTLSValidation(t *testing.T) {
	// 均未配置 → 明文连接
	b := NewBuilder(&Options{})
	if b.err != nil {
		t.Fatalf("want nil error for plaintext, got %v", b.err)
	}
	if b.dialOpts.TLSConfig != nil {
		t.Fatal("TLSConfig should be nil for plaintext")
	}

	// 仅配置 ServerName → 警告降级为明文，不报错
	b = NewBuilder(&Options{ServerName: "example.com"})
	if b.err != nil {
		t.Fatalf("want nil error for warning fallback, got %v", b.err)
	}
	if b.dialOpts.TLSConfig != nil {
		t.Fatal("TLSConfig should be nil when only ServerName is set")
	}

	// 仅配置 CAFile（文件不存在）→ 警告降级为明文，不报错
	b = NewBuilder(&Options{CAFile: "not-exist.pem"})
	if b.err != nil {
		t.Fatalf("want nil error for warning fallback, got %v", b.err)
	}
	if b.dialOpts.TLSConfig != nil {
		t.Fatal("TLSConfig should be nil when only CAFile is set")
	}

	// CAFile + ServerName 均配置 → 启用TLS
	caFile := newCAFile(t)
	b = NewBuilder(&Options{CAFile: caFile, ServerName: "localhost"})
	if b.err != nil {
		t.Fatalf("CAFile+ServerName should enable TLS, got err: %v", b.err)
	}
	if b.dialOpts.TLSConfig == nil {
		t.Fatal("TLSConfig should be set when both are configured")
	}
}

// TestBuilderInitError 验证注册中心初始化失败时错误经 Build 返回而非 fatal
func TestBuilderInitError(t *testing.T) {
	b := NewBuilder(&Options{Discovery: &failDiscovery{}})

	if _, err := b.Build("direct://127.0.0.1:8011"); err == nil {
		t.Fatal("Build should return the init error")
	}
}
