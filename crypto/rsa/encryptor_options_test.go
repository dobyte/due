package rsa

import "testing"

func TestDefaultEncryptorPadding(t *testing.T) {
	if got := defaultEncryptorOptions().padding; got != OAEP {
		t.Fatalf("default encryptor padding: got %s, want %s", got, OAEP)
	}
}
