package gate

import (
	"sync"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/core/security/handshake"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

// securityOptions configures the session security between the client and the gate.
type securityOptions struct {
	signer crypto.Signer // signer used to authenticate the gate
	suites []aead.Suite  // negotiable AEAD suites, empty means the handshake defaults
}

// securityManager holds the per-connection security state and the shared handshake server.
type securityManager struct {
	gate   *Gate
	server *handshake.Server
	states sync.Map // connection ID -> *securityState
	conns  sync.Map // connection ID -> *securedConn
}

// securityState is the security state of a single connection.
type securityState struct {
	mu      sync.Mutex
	pending *handshake.Pending
	session *handshake.Session
}

// securedConn decorates a connection so that every pushed message is sealed with the session key
// of that connection.
type securedConn struct {
	network.Conn
	manager *securityManager
}

// Push seals the payload of buf with the session key and pushes the result.
func (c *securedConn) Push(buf buffer.Buffer) error {
	return c.manager.push(c.Conn, buf)
}

// newSecurityManager creates a security manager from the gate options.
func newSecurityManager(gate *Gate, opts *securityOptions) *securityManager {
	return &securityManager{
		gate:   gate,
		server: handshake.NewServer(opts.signer, opts.suites...),
	}
}

// wrap decorates conn and records the mapping so that the teardown path can find the wrapper.
func (m *securityManager) wrap(conn network.Conn) network.Conn {
	wrapped := &securedConn{Conn: conn, manager: m}
	m.conns.Store(conn.ID(), wrapped)

	return wrapped
}

// unwrap returns the recorded wrapper of conn, or conn itself when security is disabled for it.
func (m *securityManager) unwrap(conn network.Conn) network.Conn {
	if wrapped, ok := m.conns.Load(conn.ID()); ok {
		return wrapped.(network.Conn)
	}

	return conn
}

// remove releases the state of the given connection.
func (m *securityManager) remove(cid int64) {
	m.states.Delete(cid)
	m.conns.Delete(cid)
}

// receiveHandshake handles a handshake frame received from a client.
func (m *securityManager) receiveHandshake(conn network.Conn, buf buffer.Buffer) {
	payload, err := packet.ExtractHandshake(buf)
	if err != nil {
		buf.Release()
		log.Warnf("extract handshake frame failed, cid: %d, err: %v", conn.ID(), err)
		_ = conn.Close()
		return
	}

	frame := append([]byte(nil), payload...)
	buf.Release()

	m.advanceHandshake(conn, frame)
}

// decrypt decrypts a business message received from a client and delivers the plaintext to the
// node. It reports whether the message was consumed.
func (m *securityManager) decrypt(conn network.Conn, buf buffer.Buffer) bool {
	route, seq, payload, err := packet.ExtractMessage(buf)
	if err != nil {
		buf.Release()
		log.Warnf("extract message failed, cid: %d, err: %v", conn.ID(), err)
		return true
	}

	value, ok := m.states.Load(conn.ID())
	if !ok {
		buf.Release()
		log.Warnf("session is not established, cid: %d", conn.ID())
		_ = conn.Close()
		return true
	}

	state := value.(*securityState)

	state.mu.Lock()
	session := state.session
	state.mu.Unlock()

	if session == nil {
		buf.Release()
		log.Warnf("session is not confirmed, cid: %d", conn.ID())
		_ = conn.Close()
		return true
	}

	plaintext, err := session.Open(payload, handshake.MakeAAD(handshake.ClientToServer, session.Epoch(), route, seq))
	buf.Release()

	if err != nil {
		log.Warnf("open secure message failed, cid: %d, route: %d, seq: %d, err: %v", conn.ID(), route, seq, err)
		_ = conn.Close()
		return true
	}

	packed, err := packet.PackMessage(&packet.Message{Route: route, Seq: seq, Buffer: plaintext})
	if err != nil {
		log.Errorf("pack message failed, cid: %d, err: %v", conn.ID(), err)
		return true
	}

	m.gate.proxy.deliver(m.gate.ctx, conn, packed)

	return true
}

// advanceHandshake advances the handshake state machine of the given connection.
func (m *securityManager) advanceHandshake(conn network.Conn, frame []byte) {
	value, _ := m.states.LoadOrStore(conn.ID(), &securityState{})
	state := value.(*securityState)

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.session != nil {
		return
	}

	if state.pending == nil {
		pending, reply, err := m.server.Accept(frame)
		if err != nil {
			log.Warnf("accept handshake failed, cid: %d, err: %v", conn.ID(), err)
			_ = conn.Close()
			return
		}

		state.pending = pending
		m.pushFrame(conn, reply)

		return
	}

	session, err := m.server.Confirm(state.pending, frame)
	if err != nil {
		log.Warnf("confirm handshake failed, cid: %d, err: %v", conn.ID(), err)
		_ = conn.Close()
		return
	}

	state.pending = nil
	state.session = session
}

// pushFrame pushes a plaintext handshake frame to the client.
func (m *securityManager) pushFrame(conn network.Conn, frame []byte) {
	buf, err := packet.PackHandshake(frame)
	if err != nil {
		return
	}

	if err = conn.Push(buf); err != nil {
		buf.Release()
	}
}

// push seals the payload of buf with the session key of the connection and pushes the result.
//
// The original buffer is released only after the encrypted frame is pushed successfully. On any
// failure it is left to the caller, which owns it until the push succeeds.
func (m *securityManager) push(conn network.Conn, buf buffer.Buffer) error {
	value, ok := m.states.Load(conn.ID())
	if !ok {
		return errors.New("session is not established")
	}

	state := value.(*securityState)

	state.mu.Lock()
	session := state.session
	state.mu.Unlock()

	if session == nil {
		return errors.New("session is not confirmed")
	}

	route, seq, payload, err := packet.ExtractMessage(buf)
	if err != nil {
		return err
	}

	sealed, err := session.Seal(payload, handshake.MakeAAD(handshake.ServerToClient, session.Epoch(), route, seq))
	if err != nil {
		return err
	}

	packed, err := packet.PackMessage(&packet.Message{Route: route, Seq: seq, Buffer: sealed})
	if err != nil {
		return err
	}

	if err = conn.Push(packed); err != nil {
		packed.Release()
		return err
	}

	buf.Release()

	return nil
}
