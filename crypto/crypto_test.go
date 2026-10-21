package crypto

import (
	"bytes"

	"github.com/dobyte/due/v2/log"
)

// panickingLogger is a log.Logger whose fatal methods panic instead of terminating the process.
// It lets the tests cover the fatal branches of the registry.
type panickingLogger struct {
	log.Logger
}

// Fatal panics with a fixed message.
func (l *panickingLogger) Fatal(a ...any) { panic("fatal") }

// Fatalf panics with a fixed message.
func (l *panickingLogger) Fatalf(format string, a ...any) { panic("fatal") }

// Close reports success without doing anything.
func (l *panickingLogger) Close() error { return nil }

// mockEncryptor is an in-memory Encryptor used to exercise the registry.
type mockEncryptor struct {
	name string
}

// Name returns the encryptor name.
func (m *mockEncryptor) Name() string { return m.name }

// Encrypt prefixes the data with the encryptor name.
func (m *mockEncryptor) Encrypt(data []byte) ([]byte, error) {
	return append([]byte(m.name+":"), data...), nil
}

// Decrypt strips the encryptor name prefix from the data.
func (m *mockEncryptor) Decrypt(data []byte) ([]byte, error) {
	return bytes.TrimPrefix(data, []byte(m.name+":")), nil
}

// mockSigner is an in-memory Signer used to exercise the registry.
type mockSigner struct {
	name string
}

// Name returns the signer name.
func (m *mockSigner) Name() string { return m.name }

// Sign returns the signer name followed by the data.
func (m *mockSigner) Sign(data []byte) ([]byte, error) {
	return append([]byte(m.name), data...), nil
}

// Verify reports true when the signature starts with the signer name.
func (m *mockSigner) Verify(data []byte, signature []byte) (bool, error) {
	return bytes.HasPrefix(signature, []byte(m.name)), nil
}
