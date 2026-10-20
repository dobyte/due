package handshake

import (
	"crypto/hmac"
	"crypto/sha256"
	"testing"

	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/core/security/aead/aesgcm"
	"github.com/dobyte/due/v2/core/security/aead/chacha20poly1305"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/errors"
)

// testSigner is an in-memory signer used to exercise the handshake state machine without
// depending on the standalone signature modules.
type testSigner struct {
	key []byte
}

// Name returns the name of the signer.
func (s *testSigner) Name() string { return "test" }

// Sign signs data with the test key.
func (s *testSigner) Sign(data []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(data)

	return mac.Sum(nil), nil
}

// Verify verifies the signature of data with the test key.
func (s *testSigner) Verify(data []byte, signature []byte) (bool, error) {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(data)

	return hmac.Equal(mac.Sum(nil), signature), nil
}

// newTestSignerPair returns a test signer and the verifier holding the same key.
func newTestSignerPair() (crypto.Signer, crypto.Signer) {
	signer := &testSigner{key: []byte("due-handshake-test-key")}

	return signer, signer
}

// establishSession performs a full handshake and returns the client and server sessions.
func establishSession(t testing.TB, suite aead.Suite) (*Session, *Session) {
	t.Helper()

	signer, verifier := newTestSignerPair()

	server := NewServer(signer, suite)
	client := NewClient(verifier, suite)

	hello, err := client.Hello()
	if err != nil {
		t.Fatalf("client hello failed: %v", err)
	}

	pending, serverHello, err := server.Accept(hello)
	if err != nil {
		t.Fatalf("server accept failed: %v", err)
	}

	clientSession, finished, err := client.Handle(serverHello)
	if err != nil {
		t.Fatalf("client handle failed: %v", err)
	}

	serverSession, err := server.Confirm(pending, finished)
	if err != nil {
		t.Fatalf("server confirm failed: %v", err)
	}

	if clientSession.Suite() != suite || serverSession.Suite() != suite {
		t.Fatalf("unexpected suite: client=%s server=%s want=%s", clientSession.Suite(), serverSession.Suite(), suite)
	}

	return clientSession, serverSession
}

// TestHandshake verifies the handshake and both traffic directions for every supported suite.
func TestHandshake(t *testing.T) {
	for _, suite := range []aead.Suite{aesgcm.Suite, chacha20poly1305.Suite} {
		t.Run(suite.Name(), func(t *testing.T) {
			clientSession, serverSession := establishSession(t, suite)

			uplink := MakeAAD(ClientToServer, clientSession.Epoch(), 1001, 1)
			sealed, err := clientSession.Seal([]byte("hello"), uplink)
			if err != nil {
				t.Fatalf("seal failed: %v", err)
			}

			if len(sealed) != len("hello")+clientSession.Overhead() {
				t.Fatalf("unexpected sealed size: %d", len(sealed))
			}

			plaintext, err := serverSession.Open(sealed, uplink)
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}

			if string(plaintext) != "hello" {
				t.Fatalf("unexpected plaintext: %s", plaintext)
			}

			downlink := MakeAAD(ServerToClient, serverSession.Epoch(), 1001, 1)
			sealed, err = serverSession.Seal([]byte("world"), downlink)
			if err != nil {
				t.Fatalf("seal failed: %v", err)
			}

			plaintext, err = clientSession.Open(sealed, downlink)
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}

			if string(plaintext) != "world" {
				t.Fatalf("unexpected plaintext: %s", plaintext)
			}
		})
	}
}

// TestHandshakeRejectsUnknownSigner verifies that a client with another public key rejects the
// ServerHello.
func TestHandshakeRejectsUnknownSigner(t *testing.T) {
	signer, _ := newTestSignerPair()

	server := NewServer(signer, aesgcm.Suite)
	client := NewClient(&testSigner{key: []byte("another-key")}, aesgcm.Suite)

	hello, err := client.Hello()
	if err != nil {
		t.Fatalf("client hello failed: %v", err)
	}

	_, serverHello, err := server.Accept(hello)
	if err != nil {
		t.Fatalf("server accept failed: %v", err)
	}

	if _, _, err = client.Handle(serverHello); !errors.Is(err, errors.ErrInvalidSignature) {
		t.Fatalf("expect the invalid signature error, got %v", err)
	}
}

// TestHandshakeRejectsNoCommonSuite verifies suite negotiation failures.
func TestHandshakeRejectsNoCommonSuite(t *testing.T) {
	signer, verifier := newTestSignerPair()

	server := NewServer(signer, aesgcm.Suite)
	client := NewClient(verifier, chacha20poly1305.Suite)

	hello, err := client.Hello()
	if err != nil {
		t.Fatalf("client hello failed: %v", err)
	}

	if _, _, err = server.Accept(hello); err == nil {
		t.Fatal("expect an error when no suite is shared")
	}
}

// TestHandshakeDefaultSuites verifies that a peer without an explicit suite uses the defaults.
func TestHandshakeDefaultSuites(t *testing.T) {
	_, verifier := newTestSignerPair()

	if _, err := NewClient(verifier).Hello(); err != nil {
		t.Fatalf("client hello failed: %v", err)
	}
}

// TestHandshakeRejectsInvalidSuites verifies the suite configuration validation.
func TestHandshakeRejectsInvalidSuites(t *testing.T) {
	signer, verifier := newTestSignerPair()

	if _, err := NewClient(verifier, nil).Hello(); err == nil {
		t.Fatal("expect an error for a nil suite")
	}

	suites := make([]aead.Suite, 256)
	for i := range suites {
		suites[i] = aesgcm.Suite
	}

	if _, _, err := NewServer(signer, suites...).Accept(nil); err == nil {
		t.Fatal("expect an error for too many suites")
	}
}

// TestSessionRejectsTamperedPayload verifies that a modified ciphertext is rejected.
func TestSessionRejectsTamperedPayload(t *testing.T) {
	clientSession, serverSession := establishSession(t, aesgcm.Suite)

	aad := MakeAAD(ClientToServer, clientSession.Epoch(), 1, 1)
	sealed, err := clientSession.Seal([]byte("payload"), aad)

	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	sealed[len(sealed)-1] ^= 0xff

	if _, err = serverSession.Open(sealed, aad); !errors.Is(err, errors.ErrInvalidMessage) {
		t.Fatalf("expect the invalid message error, got %v", err)
	}
}

// TestSessionRejectsWrongAAD verifies that header fields bound by the AAD cannot change.
func TestSessionRejectsWrongAAD(t *testing.T) {
	clientSession, serverSession := establishSession(t, aesgcm.Suite)

	sealed, err := clientSession.Seal([]byte("payload"), MakeAAD(ClientToServer, clientSession.Epoch(), 1, 1))
	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	if _, err = serverSession.Open(sealed, MakeAAD(ClientToServer, clientSession.Epoch(), 2, 1)); !errors.Is(err, errors.ErrInvalidMessage) {
		t.Fatalf("expect the invalid message error, got %v", err)
	}
}

// TestSessionRejectsReplay verifies the replay window.
func TestSessionRejectsReplay(t *testing.T) {
	clientSession, serverSession := establishSession(t, aesgcm.Suite)

	aad := MakeAAD(ClientToServer, clientSession.Epoch(), 1, 1)
	sealed, err := clientSession.Seal([]byte("payload"), aad)

	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	if _, err = serverSession.Open(sealed, aad); err != nil {
		t.Fatalf("first open failed: %v", err)
	}

	if _, err = serverSession.Open(sealed, aad); !errors.Is(err, ErrReplayMessage) {
		t.Fatalf("expect the replay error, got %v", err)
	}
}

// TestSessionAcceptsOutOfOrder verifies that messages inside the window are accepted out of order
// while messages outside it are rejected.
func TestSessionAcceptsOutOfOrder(t *testing.T) {
	clientSession, serverSession := establishSession(t, aesgcm.Suite)

	aad := MakeAAD(ClientToServer, clientSession.Epoch(), 1, 1)
	sealed := make([][]byte, 0, 70)

	for i := 0; i < 70; i++ {
		data, err := clientSession.Seal([]byte("payload"), aad)
		if err != nil {
			t.Fatalf("seal failed: %v", err)
		}

		sealed = append(sealed, data)
	}

	for _, index := range []int{69, 68, 40, 10, 6} {
		if _, err := serverSession.Open(sealed[index], aad); err != nil {
			t.Fatalf("open message %d failed: %v", index, err)
		}
	}

	if _, err := serverSession.Open(sealed[68], aad); !errors.Is(err, ErrReplayMessage) {
		t.Fatalf("expect the replay error for a duplicate message, got %v", err)
	}

	for _, index := range []int{5, 0} {
		if _, err := serverSession.Open(sealed[index], aad); !errors.Is(err, ErrReplayMessage) {
			t.Fatalf("expect the replay error for the stale message %d, got %v", index, err)
		}
	}
}
