package ecc_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dobyte/due/crypto/ecc/v2"
)

func TestGenerateKey(t *testing.T) {
	key, err := ecc.GenerateKey(ecc.P256)
	if err != nil {
		t.Fatal(err)
	}

	v, err := key.MarshalPublicKey()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(v))
}

func TestKeySaveKeyPairPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce Unix permission bits")
	}

	key, err := ecc.GenerateKey(ecc.P256)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), "keys")
	if err = key.SaveKeyPair(dir, "key.pem"); err != nil {
		t.Fatal(err)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("private key directory permissions: got %o, want 700", got)
	}

	info, err := os.Stat(filepath.Join(dir, "key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("private key permissions: got %o, want 600", got)
	}
}

func TestKey_SaveKeyPair(t *testing.T) {
	key, err := ecc.GenerateKey(ecc.P256)
	if err != nil {
		t.Fatal(err)
	}

	err = key.SaveKeyPair("./pem", "key.pem")
	if err != nil {
		t.Fatal(err)
	}
}
