package client

import (
	"crypto/hmac"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/security/aead/aesgcm"
	"github.com/dobyte/due/v2/core/security/handshake"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
)

// clientSecuritySigner is an in-memory signer used by the session security tests.
type clientSecuritySigner struct {
	key []byte
}

// Name returns the name of the signer.
func (s *clientSecuritySigner) Name() string { return "test" }

// Sign signs data with the test key.
func (s *clientSecuritySigner) Sign(data []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(data)

	return mac.Sum(nil), nil
}

// Verify verifies the signature of data with the test key.
func (s *clientSecuritySigner) Verify(data []byte, signature []byte) (bool, error) {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(data)

	return hmac.Equal(mac.Sum(nil), signature), nil
}

// nextHandshakeFrame returns the payload of the next handshake frame pushed by the client.
func nextHandshakeFrame(conn *tConn) ([]byte, error) {
	select {
	case buf := <-conn.pushed:
		payload, err := packet.ExtractHandshake(buf)
		if err != nil {
			buf.Release()
			return nil, err
		}

		frame := append([]byte(nil), payload...)
		buf.Release()

		return frame, nil
	case <-time.After(5 * time.Second):
		return nil, errors.New("timeout waiting for a handshake frame")
	}
}

// TestClientSessionSecurity verifies the client side handshake, the uplink sealing and the
// downlink decryption.
func TestClientSessionSecurity(t *testing.T) {
	var (
		signer = &clientSecuritySigner{key: []byte("client-security-test")}
		conn   = newTConn(1)
		net    = &tNetwork{conn: conn}
	)

	received := make(chan string, 1)

	c := NewClient(
		WithClient(net),
		WithCodec(json.DefaultCodec),
		WithSessionSecurity(signer),
	)
	c.Proxy().AddRouteHandler(11, func(ctx *Context) {
		received <- string(ctx.Data().(buffer.Buffer).Bytes())
	})
	c.Init()
	c.Start()

	t.Cleanup(func() {
		c.Close()
		c.conns.Clear()
		c.Destroy()
	})

	type handshakeResult struct {
		session *handshake.Session
		err     error
	}

	results := make(chan handshakeResult, 1)
	server := handshake.NewServer(signer, aesgcm.Suite)

	go func() {
		hello, err := nextHandshakeFrame(conn)
		if err != nil {
			results <- handshakeResult{err: err}
			return
		}

		pending, reply, err := server.Accept(hello)
		if err != nil {
			results <- handshakeResult{err: err}
			return
		}

		buf, err := packet.PackHandshake(reply)
		if err != nil {
			results <- handshakeResult{err: err}
			return
		}

		c.handleHandshake(conn, buf)

		finished, err := nextHandshakeFrame(conn)
		if err != nil {
			results <- handshakeResult{err: err}
			return
		}

		session, err := server.Confirm(pending, finished)
		results <- handshakeResult{session: session, err: err}
	}()

	cc, err := c.Proxy().Dial()
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	var result handshakeResult

	select {
	case result = <-results:
	case <-time.After(5 * time.Second):
		t.Fatal("the server side handshake did not finish")
	}

	if result.err != nil {
		t.Fatalf("the server side handshake failed: %v", result.err)
	}

	session := result.session

	// An uplink without payload must be sealed too, otherwise the gate rejects the empty payload
	// and closes the connection.
	if err = cc.Push(&cluster.Message{Route: 10, Seq: 1}); err != nil {
		t.Fatalf("push failed: %v", err)
	}

	raw := <-conn.pushed
	route, seq, payload, err := packet.ExtractMessage(raw)
	if err != nil {
		raw.Release()
		t.Fatalf("extract message failed: %v", err)
	}

	plaintext, err := session.Open(payload, handshake.MakeAAD(handshake.ClientToServer, session.Epoch(), route, seq))
	raw.Release()

	if err != nil {
		t.Fatalf("open empty uplink failed: %v", err)
	}
	if route != 10 || seq != 1 {
		t.Fatalf("unexpected uplink route/seq: %d/%d", route, seq)
	}
	if len(plaintext) != 0 {
		t.Fatalf("unexpected uplink plaintext: %q", plaintext)
	}

	// A regular uplink must be sealed as well.
	if err = cc.Push(&cluster.Message{Route: 10, Seq: 2, Data: []byte("uplink")}); err != nil {
		t.Fatalf("push failed: %v", err)
	}

	raw = <-conn.pushed
	route, seq, payload, err = packet.ExtractMessage(raw)
	if err != nil {
		raw.Release()
		t.Fatalf("extract message failed: %v", err)
	}

	plaintext, err = session.Open(payload, handshake.MakeAAD(handshake.ClientToServer, session.Epoch(), route, seq))
	raw.Release()

	if err != nil {
		t.Fatalf("open uplink failed: %v", err)
	}
	if string(plaintext) != "uplink" {
		t.Fatalf("unexpected uplink plaintext: %q", plaintext)
	}

	// A valid downlink must be decrypted before the route handler runs.
	sealed, err := session.Seal([]byte("downlink"), handshake.MakeAAD(handshake.ServerToClient, session.Epoch(), 11, 1))
	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	buf, err := packet.PackMessage(&packet.Message{Route: 11, Seq: 1, Buffer: sealed})
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}

	c.handleReceive(conn, buf)

	select {
	case data := <-received:
		if data != "downlink" {
			t.Fatalf("unexpected downlink plaintext: %s", data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the downlink was not delivered")
	}

	// A tampered downlink must be dropped instead of reaching the route handler.
	sealed, err = session.Seal([]byte("tampered"), handshake.MakeAAD(handshake.ServerToClient, session.Epoch(), 11, 2))
	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	sealed[len(sealed)-1] ^= 0xff

	buf, err = packet.PackMessage(&packet.Message{Route: 11, Seq: 2, Buffer: sealed})
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}

	c.handleReceive(conn, buf)

	select {
	case data := <-received:
		t.Fatalf("a tampered downlink must be dropped, got %q", data)
	case <-time.After(200 * time.Millisecond):
	}
}
