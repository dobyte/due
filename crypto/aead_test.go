package crypto_test

import (
	"testing"

	"github.com/dobyte/due/v2/crypto"

	_ "github.com/dobyte/due/v2/crypto/aesgcm"
	_ "github.com/dobyte/due/v2/crypto/chacha20poly1305"
)

// TestInvokeAEADSuite verifies that the built-in AEAD suites are registered.
func TestInvokeAEADSuite(t *testing.T) {
	for _, id := range []uint8{1, 2} {
		suite, ok := crypto.InvokeAEADSuite(id)
		if !ok {
			t.Fatalf("aead suite %d is not registered", id)
		}

		if suite.ID != id || suite.Name == "" || suite.KeySize == 0 || suite.New == nil {
			t.Fatalf("unexpected aead suite: %+v", suite)
		}

		if _, err := suite.New(make([]byte, suite.KeySize)); err != nil {
			t.Fatalf("build aead cipher failed: %v", err)
		}
	}
}
