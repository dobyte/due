package gate

import (
	"crypto/hmac"
	"crypto/sha256"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/security/aead/aesgcm"
	"github.com/dobyte/due/v2/core/security/handshake"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

// securitySigner is an in-memory signer used by the security tests.
type securitySigner struct {
	key []byte
}

// Name returns the name of the signer.
func (s *securitySigner) Name() string { return "test" }

// Sign signs data with the test key.
func (s *securitySigner) Sign(data []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(data)

	return mac.Sum(nil), nil
}

// Verify verifies the signature of data with the test key.
func (s *securitySigner) Verify(data []byte, signature []byte) (bool, error) {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(data)

	return hmac.Equal(mac.Sum(nil), signature), nil
}

// securityConn is a network connection stub that captures pushed messages.
type securityConn struct {
	id     int64
	uid    atomic.Int64
	closed atomic.Bool
	pushed chan []byte
}

// ID returns the connection ID.
func (c *securityConn) ID() int64 {
	if c.id != 0 {
		return c.id
	}
	return 1
}

// UID returns the user ID.
func (c *securityConn) UID() int64 { return c.uid.Load() }

// Attr returns the attribute interface.
func (c *securityConn) Attr() network.Attr { return nil }

// Bind binds the user ID.
func (c *securityConn) Bind(uid int64) error {
	c.uid.Store(uid)
	return nil
}

// Unbind unbinds the user ID.
func (c *securityConn) Unbind() error {
	c.uid.Store(0)
	return nil
}

// Push records the pushed message.
func (c *securityConn) Push(buf buffer.Buffer) error {
	c.pushed <- append([]byte(nil), buf.Bytes()...)
	buf.Release()

	return nil
}

// State returns the connection state.
func (c *securityConn) State() network.ConnState { return network.ConnOpened }

// Close closes the connection.
func (c *securityConn) Close(force ...bool) error {
	c.closed.Store(true)
	return nil
}

// LocalIP returns the local IP address.
func (c *securityConn) LocalIP() (string, error) { return "127.0.0.1", nil }

// LocalAddr returns the local address.
func (c *securityConn) LocalAddr() (net.Addr, error) { return nil, nil }

// RemoteIP returns the remote IP address.
func (c *securityConn) RemoteIP() (string, error) { return "10.0.0.1", nil }

// RemoteAddr returns the remote address.
func (c *securityConn) RemoteAddr() (net.Addr, error) { return nil, nil }

// next returns the next pushed message.
func (c *securityConn) next(t *testing.T) []byte {
	t.Helper()

	select {
	case data := <-c.pushed:
		return data
	default:
		t.Fatal("expect a pushed message")
		return nil
	}
}

func TestSecurityHandshakeTimeout(t *testing.T) {
	signer := &securitySigner{key: []byte("gate-security-timeout-test")}
	g := NewGate(
		WithSessionSecurity(signer),
		WithSessionSecurityHandshakeTimeout(10*time.Millisecond),
	)
	conn := &securityConn{pushed: make(chan []byte, 1)}
	g.security.wrap(conn)
	t.Cleanup(func() { g.security.remove(conn.ID()) })

	deadline := time.NewTimer(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()

	for !conn.closed.Load() {
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("connection was not closed after the handshake timeout")
		}
	}
}

// TestSessionSecurity verifies the handshake, the downlink sealing and the uplink rejection.
func TestSessionSecurity(t *testing.T) {
	signer := &securitySigner{key: []byte("gate-security-test")}

	g := NewGate(WithSessionSecurity(signer))
	conn := &securityConn{pushed: make(chan []byte, 4)}
	wrapped := g.security.wrap(conn)

	client := handshake.NewClient(signer, aesgcm.Suite)
	hello, err := client.Hello()
	if err != nil {
		t.Fatalf("client hello failed: %v", err)
	}

	injectHandshake(t, g, conn, hello)

	session, finished, err := client.Handle(nextHandshake(t, conn.next(t)))
	if err != nil {
		t.Fatalf("client handle failed: %v", err)
	}

	injectHandshake(t, g, conn, finished)

	// The downlink payload must be sealed and openable by the client session.
	packed, err := packet.PackMessage(&packet.Message{Route: 10, Seq: 1, Buffer: []byte("downlink")})
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}

	if err = wrapped.Push(packed); err != nil {
		t.Fatalf("secured push failed: %v", err)
	}

	raw := buffer.NewBytes(conn.next(t), true)
	route, seq, payload, err := packet.ExtractMessage(raw)
	if err != nil {
		t.Fatalf("extract message failed: %v", err)
	}

	plaintext, err := session.Open(payload, handshake.MakeAAD(handshake.ServerToClient, session.Epoch(), route, seq))
	if err != nil {
		t.Fatalf("open downlink failed: %v", err)
	}

	if string(plaintext) != "downlink" {
		t.Fatalf("unexpected downlink plaintext: %s", plaintext)
	}

	// A valid uplink must be authenticated and accepted without closing the connection.
	sealed, err := session.Seal([]byte("uplink"), handshake.MakeAAD(handshake.ClientToServer, session.Epoch(), 10, 2))
	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	injectMessage(t, g, conn, 10, 2, sealed)

	if conn.closed.Load() {
		t.Fatal("the connection must stay open after a valid uplink")
	}

	// A tampered uplink must close the connection.
	sealed, err = session.Seal([]byte("uplink"), handshake.MakeAAD(handshake.ClientToServer, session.Epoch(), 10, 3))
	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	sealed[len(sealed)-1] ^= 0xff

	injectMessage(t, g, conn, 10, 3, sealed)

	if !conn.closed.Load() {
		t.Fatal("the connection must be closed after a tampered uplink")
	}
}

// nextHandshake extracts the payload of the next pushed handshake frame.
func nextHandshake(t *testing.T, packed []byte) []byte {
	t.Helper()

	payload, err := packet.ExtractHandshake(buffer.NewBytes(packed, true))
	if err != nil {
		t.Fatalf("extract handshake frame failed: %v", err)
	}

	return payload
}

// injectHandshake packs and delivers a handshake frame.
func injectHandshake(t *testing.T, g *Gate, conn network.Conn, frame []byte) {
	t.Helper()

	buf, err := packet.PackHandshake(frame)
	if err != nil {
		t.Fatalf("pack handshake frame failed: %v", err)
	}

	g.security.receiveHandshake(conn, buf)
}

// injectMessage packs and delivers a business message to the gate security manager.
func injectMessage(t *testing.T, g *Gate, conn network.Conn, route, seq int32, payload []byte) {
	t.Helper()

	buf, err := packet.PackMessage(&packet.Message{Route: route, Seq: seq, Buffer: payload})
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}

	g.security.decrypt(conn, buf)
}
