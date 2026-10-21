package rsa_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dobyte/due/crypto/rsa/v2"
)

func TestGenerateKey(t *testing.T) {
	key, err := rsa.GenerateKey(2048)
	if err != nil {
		t.Fatal(err)
	}

	v, err := key.MarshalPublicKey(rsa.PKCS1)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(v))
}

func TestKeySaveKeyPairPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce Unix permission bits")
	}

	key, err := rsa.GenerateKey(2048)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	privateKey := filepath.Join(dir, "key.pem")
	if err = os.WriteFile(privateKey, []byte("old key"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err = key.SaveKeyPair(rsa.PKCS1, dir, "key.pem"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("private key permissions: got %o, want 600", got)
	}
}

func TestKey_SaveKeyPair(t *testing.T) {
	key, err := rsa.GenerateKey(2048)
	if err != nil {
		t.Fatal(err)
	}

	err = key.SaveKeyPair(rsa.PKCS1, "./pem", "key.pem")
	if err != nil {
		t.Fatal(err)
	}
}
