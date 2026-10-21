// Package securitytest provides an encrypted echo peer for benchmark regression tests.
package securitytest

import (
	"sync"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/security/handshake"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

// StartEcho starts a peer which requires authenticated encryption when secure is true.
// The caller must close its clients before the test cleanup stops this server.
func StartEcho(t *testing.T, server network.Server, signer crypto.Signer, secure bool) {
	t.Helper()
	type state struct {
		pending *handshake.Pending
		session *handshake.Session
	}
	var states sync.Map
	handshaker := handshake.NewServer(signer)
	push := func(conn network.Conn, buf buffer.Buffer, err error) {
		if err != nil {
			t.Errorf("pack reply: %v", err)
			return
		}
		if err := conn.Push(buf); err != nil {
			buf.Release()
			t.Errorf("push reply: %v", err)
		}
	}
	server.OnConnect(func(conn network.Conn) { states.Store(conn.ID(), &state{}) })
	server.OnDisconnect(func(conn network.Conn) { states.Delete(conn.ID()) })
	server.OnHandshake(func(conn network.Conn, buf buffer.Buffer) {
		defer buf.Release()
		if !secure {
			t.Error("plaintext peer received a security handshake")
			return
		}
		value, ok := states.Load(conn.ID())
		if !ok {
			t.Error("handshake without connection state")
			return
		}
		st := value.(*state)
		frame, err := packet.ExtractHandshake(buf)
		if err != nil {
			t.Errorf("extract handshake: %v", err)
			return
		}
		if st.pending == nil {
			pending, reply, err := handshaker.Accept(frame)
			if err != nil {
				t.Errorf("accept handshake: %v", err)
				return
			}
			st.pending = pending
			packed, err := packet.PackHandshake(reply)
			push(conn, packed, err)
			return
		}
		st.session, err = handshaker.Confirm(st.pending, frame)
		if err != nil {
			t.Errorf("confirm handshake: %v", err)
		}
	})
	server.OnReceive(func(conn network.Conn, buf buffer.Buffer) {
		route, seq, payload, err := packet.ExtractMessage(buf)
		if err != nil {
			buf.Release()
			t.Errorf("extract message: %v", err)
			return
		}
		if !secure {
			packed, err := packet.PackMessage(route, seq, append([]byte(nil), payload...))
			buf.Release()
			push(conn, packed, err)
			return
		}
		value, ok := states.Load(conn.ID())
		if !ok {
			buf.Release()
			t.Error("message without connection state")
			return
		}
		session := value.(*state).session
		if session == nil {
			buf.Release()
			t.Error("plaintext sent before security handshake completed")
			return
		}
		plain, err := session.OpenMessage(nil, payload, handshake.ClientToServer, route, seq)
		buf.Release()
		if err != nil {
			t.Errorf("decrypt request: %v", err)
			return
		}
		sealed, err := session.SealMessage(nil, plain, handshake.ServerToClient, route, seq)
		if err != nil {
			t.Errorf("encrypt reply: %v", err)
			return
		}
		packed, err := packet.PackMessage(route, seq, sealed)
		push(conn, packed, err)
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Errorf("stop echo peer: %v", err)
		}
	})
}
