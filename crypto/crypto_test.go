package crypto

import (
	"bytes"
	"testing"

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

// TestRegisterAndInvokeEncryptor verifies the encryptor registration and lookup round trip.
func TestRegisterAndInvokeEncryptor(t *testing.T) {
	encryptor := &mockEncryptor{name: "mock-encryptor"}
	RegisterEncryptor(encryptor)

	got := InvokeEncryptor("mock-encryptor")
	if got != Encryptor(encryptor) {
		t.Fatal("expect the registered encryptor to be returned")
	}

	encrypted, err := got.Encrypt([]byte("data"))
	if err != nil {
		t.Fatalf("encrypt failed, err: %v", err)
	}
	decrypted, err := got.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt failed, err: %v", err)
	}
	if string(decrypted) != "data" {
		t.Errorf("invalid decrypted data, expect: data, actual: %s", decrypted)
	}
}

// TestRegisterEncryptorOverwrite verifies that registering the same name again overwrites the
// previous encryptor.
func TestRegisterEncryptorOverwrite(t *testing.T) {
	first := &mockEncryptor{name: "mock-overwrite-encryptor"}
	second := &mockEncryptor{name: "mock-overwrite-encryptor"}

	RegisterEncryptor(first)
	RegisterEncryptor(second)

	if got := InvokeEncryptor("mock-overwrite-encryptor"); got != Encryptor(second) {
		t.Error("expect the newly registered encryptor to win")
	}
}

// TestRegisterEncryptorNil verifies that registering a nil encryptor is fatal.
func TestRegisterEncryptorNil(t *testing.T) {
	orig := log.GetLogger()
	log.SetLogger(&panickingLogger{})
	defer log.SetLogger(orig)

	defer func() {
		if recover() == nil {
			t.Error("expect a panic when registering a nil encryptor")
		}
	}()

	RegisterEncryptor(nil)
}

// TestRegisterEncryptorWithoutName verifies that registering an encryptor without a name is fatal.
func TestRegisterEncryptorWithoutName(t *testing.T) {
	orig := log.GetLogger()
	log.SetLogger(&panickingLogger{})
	defer log.SetLogger(orig)

	defer func() {
		if recover() == nil {
			t.Error("expect a panic when registering an encryptor without a name")
		}
	}()

	RegisterEncryptor(&mockEncryptor{name: ""})
}

// TestInvokeEncryptorUnregistered verifies that looking up an unknown encryptor is fatal.
func TestInvokeEncryptorUnregistered(t *testing.T) {
	orig := log.GetLogger()
	log.SetLogger(&panickingLogger{})
	defer log.SetLogger(orig)

	defer func() {
		if recover() == nil {
			t.Error("expect a panic when invoking an unregistered encryptor")
		}
	}()

	InvokeEncryptor("mock-missing-encryptor")
}

// TestRegisterAndInvokeSigner verifies the signer registration and lookup round trip.
func TestRegisterAndInvokeSigner(t *testing.T) {
	signer := &mockSigner{name: "mock-signer"}
	RegisterSigner(signer)

	got := InvokeSigner("mock-signer")
	if got != Signer(signer) {
		t.Fatal("expect the registered signer to be returned")
	}

	signature, err := got.Sign([]byte("data"))
	if err != nil {
		t.Fatalf("sign failed, err: %v", err)
	}
	ok, err := got.Verify([]byte("data"), signature)
	if err != nil {
		t.Fatalf("verify failed, err: %v", err)
	}
	if !ok {
		t.Error("expect the signature to be verified")
	}
}

// TestRegisterSignerOverwrite verifies that registering the same name again overwrites the
// previous signer.
func TestRegisterSignerOverwrite(t *testing.T) {
	first := &mockSigner{name: "mock-overwrite-signer"}
	second := &mockSigner{name: "mock-overwrite-signer"}

	RegisterSigner(first)
	RegisterSigner(second)

	if got := InvokeSigner("mock-overwrite-signer"); got != Signer(second) {
		t.Error("expect the newly registered signer to win")
	}
}

// TestRegisterSignerNil verifies that registering a nil signer is fatal.
func TestRegisterSignerNil(t *testing.T) {
	orig := log.GetLogger()
	log.SetLogger(&panickingLogger{})
	defer log.SetLogger(orig)

	defer func() {
		if recover() == nil {
			t.Error("expect a panic when registering a nil signer")
		}
	}()

	RegisterSigner(nil)
}

// TestRegisterSignerWithoutName verifies that registering a signer without a name is fatal.
func TestRegisterSignerWithoutName(t *testing.T) {
	orig := log.GetLogger()
	log.SetLogger(&panickingLogger{})
	defer log.SetLogger(orig)

	defer func() {
		if recover() == nil {
			t.Error("expect a panic when registering a signer without a name")
		}
	}()

	RegisterSigner(&mockSigner{name: ""})
}

// TestInvokeSignerUnregistered verifies that looking up an unknown signer is fatal.
func TestInvokeSignerUnregistered(t *testing.T) {
	orig := log.GetLogger()
	log.SetLogger(&panickingLogger{})
	defer log.SetLogger(orig)

	defer func() {
		if recover() == nil {
			t.Error("expect a panic when invoking an unregistered signer")
		}
	}()

	InvokeSigner("mock-missing-signer")
}
