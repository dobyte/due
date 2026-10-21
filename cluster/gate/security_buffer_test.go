package gate

import (
	"bytes"
	"sync"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/security/aead/aesgcm"
	"github.com/dobyte/due/v2/core/security/handshake"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

// queuedSecurityConn retains ownership until the test models the network writer's release.
type queuedSecurityConn struct {
	*securityConn
	mu     sync.Mutex
	queued []buffer.Buffer
	fail   bool
}

func (c *queuedSecurityConn) Push(buf buffer.Buffer) error {
	c.mu.Lock()
	if c.fail {
		c.mu.Unlock()
		return errors.ErrWriteTimeout
	}
	c.queued = append(c.queued, buf)
	c.mu.Unlock()
	return nil
}

func newSecurityTestPeer(t testing.TB, id int64) (*Gate, *queuedSecurityConn, network.Conn, *handshake.Session) {
	t.Helper()
	signer := &securitySigner{key: []byte("security-buffer-test")}
	g := NewGate(WithSessionSecurity(signer))
	base := &securityConn{id: id, pushed: make(chan []byte, 1)}
	conn := &queuedSecurityConn{securityConn: base}
	wrapped := g.security.wrap(conn)
	client := handshake.NewClient(signer, aesgcm.Suite)
	hello, err := client.Hello()
	if err != nil {
		t.Fatal(err)
	}
	// The base stub captures handshake frames; business frames stay in the queued connection.
	g.security.advanceHandshake(base, hello)
	reply, err := packet.ExtractHandshake(buffer.NewBytes(<-base.pushed, true))
	if err != nil {
		t.Fatal(err)
	}
	session, finished, err := client.Handle(reply)
	if err != nil {
		t.Fatal(err)
	}
	g.security.advanceHandshake(base, finished)
	return g, conn, wrapped, session
}

// TestSecuritySharedPush verifies that concurrent recipients neither mutate shared plaintext nor
// release ciphertext while it is still queued. Each recipient must use its own session key.
func TestSecuritySharedPush(t *testing.T) {
	const recipients = 8
	payload := []byte("shared plaintext")
	packed, err := packet.PackMessage(10, 1, payload)
	if err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(packed.Bytes())
	packed.Delay(recipients + 1)
	conns := make([]*queuedSecurityConn, recipients)
	wrapped := make([]network.Conn, recipients)
	sessions := make([]*handshake.Session, recipients)
	for i := range conns {
		_, conns[i], wrapped[i], sessions[i] = newSecurityTestPeer(t, int64(i+1))
	}
	var wg sync.WaitGroup
	for _, conn := range wrapped {
		wg.Go(func() {
			if err := conn.Push(packed); err != nil {
				t.Error(err)
				packed.Release()
			}
		})
	}
	wg.Wait()
	if !bytes.Equal(packed.Bytes(), original) {
		t.Fatal("shared plaintext was changed")
	}
	packed.Release()
	// Churn the same size class before consuming queued frames to expose premature pool returns.
	for range 128 {
		buf := buffer.MallocBytes(len(payload) + sessions[0].Overhead())
		copy(buf.Bytes(), bytes.Repeat([]byte{0xff}, buf.Len()))
		buf.Release()
	}
	seen := make(map[*byte]bool)
	for i, conn := range conns {
		if len(conn.queued) != 1 {
			t.Fatalf("recipient %d has %d queued frames", i, len(conn.queued))
		}
		buf := conn.queued[0]
		if buf.Nodes() != 1 {
			t.Fatalf("recipient %d has %d frame nodes, want 1", i, buf.Nodes())
		}
		route, seq, sealed, err := packet.ExtractMessage(buf)
		if err != nil {
			t.Fatal(err)
		}
		if seen[&sealed[0]] {
			t.Fatal("recipients share mutable ciphertext storage")
		}
		seen[&sealed[0]] = true
		plain, err := sessions[i].OpenMessage(nil, sealed, handshake.ServerToClient, route, seq)
		if err != nil || !bytes.Equal(plain, payload) {
			t.Fatalf("recipient %d: plaintext=%q err=%v", i, plain, err)
		}
		buf.Release()
	}
}

// TestSecurityPushFailure verifies that write failure preserves the caller's original buffer,
// retry remains possible, and an old wrapper cannot send after security state is removed.
func TestSecurityPushFailure(t *testing.T) {
	g, conn, wrapped, client := newSecurityTestPeer(t, 1)
	buf, err := packet.PackMessage(10, 1, []byte("retry"))
	if err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(buf.Bytes())
	conn.fail = true
	if err = wrapped.Push(buf); !errors.Is(err, errors.ErrWriteTimeout) || !bytes.Equal(buf.Bytes(), original) {
		t.Fatalf("failure consumed the original: %v", err)
	}
	conn.fail = false
	if err = wrapped.Push(buf); err != nil {
		t.Fatal(err)
	}
	queued := conn.queued[0]
	route, seq, sealed, err := packet.ExtractMessage(queued)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := client.OpenMessage(nil, sealed, handshake.ServerToClient, route, seq)
	if err != nil || string(plain) != "retry" {
		t.Fatalf("retry failed: plaintext=%q err=%v", plain, err)
	}
	queued.Release()
	g.security.remove(conn.ID())
	buf, err = packet.PackMessage(10, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = wrapped.Push(buf); err == nil {
		t.Fatal("removed security session is still usable")
	}
	buf.Release()
	g.security.advanceHandshake(conn, nil)
	if _, ok := g.security.states.Load(conn.ID()); ok {
		t.Fatal("late handshake recreated removed state")
	}
}

// TestSecurityEmptyCiphertext rejects a plaintext empty business frame without a pool panic.
func TestSecurityEmptyCiphertext(t *testing.T) {
	g, conn, _, _ := newSecurityTestPeer(t, 1)
	buf, err := packet.PackMessage(10, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	g.security.decrypt(conn, buf)
	if !conn.closed.Load() {
		t.Fatal("empty ciphertext must close the connection")
	}
}
