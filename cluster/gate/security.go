package gate

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/core/security/handshake"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xcall"
)

// securityOptions configures the session security between the client and the gate.
type securityOptions struct {
	signer crypto.Signer // signer used to authenticate the gate
	suites []aead.Suite  // negotiable AEAD suites, empty means the handshake defaults
}

// securityManager holds the per-connection security state and the shared handshake server.
type securityManager struct {
	gate     *Gate
	server   *handshake.Server
	timeout  time.Duration
	states   sync.Map       // connection ID -> *securityState
	conns    sync.Map       // connection ID -> *securedConn
	closerMu sync.Mutex     // serializes the closer registration with shutdown
	draining bool           // protected by closerMu; reports that new closes are no longer accepted
	closers  sync.WaitGroup // security close goroutines, joined by shutdown
}

// securityState is the security state of a single connection.
type securityState struct {
	mu      sync.Mutex
	pending *handshake.Pending
	session atomic.Pointer[handshake.Session]
	closed  bool        // protected by mu
	closing bool        // protected by mu; at most one security close goroutine per state
	timer   *time.Timer // protected by mu
}

// securedConn decorates a connection so that every pushed message is sealed with the session key
// of that connection.
type securedConn struct {
	network.Conn
	manager *securityManager
	state   *securityState
	pushMu  sync.Mutex
}

// Push seals the payload of buf with the session key and pushes the result.
func (c *securedConn) Push(buf buffer.Buffer) error {
	c.pushMu.Lock()
	err := c.manager.push(c.Conn, c.state, buf)
	c.pushMu.Unlock()

	return err
}

// newSecurityManager creates a security manager from the gate options.
func newSecurityManager(gate *Gate, opts *securityOptions, timeout time.Duration) *securityManager {
	return &securityManager{
		gate:    gate,
		server:  handshake.NewServer(opts.signer, opts.suites...),
		timeout: timeout,
	}
}

// wrap decorates conn and records the mapping so that the teardown path can find the wrapper.
func (m *securityManager) wrap(conn network.Conn) network.Conn {
	cid := conn.ID()
	state := &securityState{}
	value, loaded := m.states.LoadOrStore(cid, state)
	state = value.(*securityState)
	wrapped := &securedConn{Conn: conn, manager: m, state: state}
	m.conns.Store(cid, wrapped)
	if !loaded && m.timeout > 0 {
		state.mu.Lock()
		if !state.closed && state.session.Load() == nil {
			state.timer = time.AfterFunc(m.timeout, func() { m.expire(cid) })
		}
		state.mu.Unlock()
	}

	return wrapped
}

// unwrap returns the recorded wrapper of conn, or conn itself when security is disabled for it.
func (m *securityManager) unwrap(conn network.Conn) network.Conn {
	if wrapped, ok := m.conns.Load(conn.ID()); ok {
		return wrapped.(network.Conn)
	}

	return conn
}

// confirmed reports whether the security handshake of the given connection was confirmed.
func (m *securityManager) confirmed(cid int64) bool {
	if value, ok := m.states.Load(cid); ok {
		return value.(*securityState).session.Load() != nil
	}

	return false
}

// remove releases the state of the given connection.
func (m *securityManager) remove(cid int64) {
	if value, ok := m.states.LoadAndDelete(cid); ok {
		state := value.(*securityState)
		state.mu.Lock()
		state.closed = true
		if state.timer != nil {
			state.timer.Stop()
			state.timer = nil
		}
		state.pending = nil
		state.session.Store(nil)
		state.mu.Unlock()
	}
	m.conns.Delete(cid)
}

// expire closes a connection that did not complete the security handshake in time.
func (m *securityManager) expire(cid int64) {
	value, ok := m.conns.Load(cid)
	if !ok {
		return
	}

	conn := value.(*securedConn)
	state := conn.state
	state.mu.Lock()
	if !state.closed && state.session.Load() == nil {
		m.closeConnLocked(cid, conn)
	}
	state.mu.Unlock()
}

// closeConn requests at most one forced close for the current security state. It runs independently
// of the global task pool so a receive callback can return before Close waits for the reader.
func (m *securityManager) closeConn(cid int64) {
	value, ok := m.conns.Load(cid)
	if !ok {
		return
	}
	conn := value.(*securedConn)
	state := conn.state
	state.mu.Lock()
	m.closeConnLocked(cid, conn)
	state.mu.Unlock()
}

// closeConnLocked claims a close while the caller holds conn.state.mu. Expiry uses the same lock
// for its confirmation check so it cannot close a session that finishes its handshake meanwhile.
func (m *securityManager) closeConnLocked(cid int64, conn *securedConn) {
	state := conn.state
	current, active := m.states.Load(cid)
	if !active || current != state || state.closing {
		return
	}
	if current, ok := m.conns.Load(cid); !ok || current != conn {
		return
	}
	state.closing = true
	state.closed = true
	state.pending = nil
	if state.timer != nil {
		state.timer.Stop()
		state.timer = nil
	}
	m.startCloser(cid, conn)
}

// startCloser launches the close goroutine of a claimed connection unless the manager is draining.
//
// The registration is serialized with [securityManager.shutdown], so the wait group is never added
// to while it is being waited on. A close rejected during the drain does not leave a connection
// open: the network server has already closed every connection by then.
func (m *securityManager) startCloser(cid int64, conn *securedConn) {
	m.closerMu.Lock()
	if m.draining {
		m.closerMu.Unlock()
		return
	}
	m.closers.Add(1)
	m.closerMu.Unlock()

	xcall.Go(func() {
		defer m.closers.Done()
		m.closeSecureConn(cid, conn)
	})
}

// shutdown stops accepting new security closes, releases the pending handshake timers and waits for
// the close goroutines started before it.
//
// Waiting on the closers after the network server stopped is not enough on its own: a connection
// closed by the transport keeps its handshake timer until the disconnect callback runs, which may
// still be queued in the task pool, so a late expiry could register a closer concurrently with the
// wait. The drain flag rejects those registrations instead.
func (m *securityManager) shutdown() {
	m.closerMu.Lock()
	m.draining = true
	m.closerMu.Unlock()

	m.states.Range(func(_, value any) bool {
		state := value.(*securityState)
		state.mu.Lock()
		state.closed = true
		if state.timer != nil {
			state.timer.Stop()
			state.timer = nil
		}
		state.mu.Unlock()

		return true
	})

	m.closers.Wait()
}

// closeSecureConn discards work for a removed or replaced security state. The check narrows the
// reuse window but cannot make it atomic with Close through the existing network.Conn interface.
func (m *securityManager) closeSecureConn(cid int64, conn *securedConn) {
	value, ok := m.states.Load(cid)
	if !ok || value != conn.state {
		return
	}
	if current, ok := m.conns.Load(cid); !ok || current != conn {
		return
	}
	if err := conn.Conn.Close(true); err != nil && !errors.Is(err, errors.ErrConnectionClosed) {
		log.Warnf("close secure connection failed, cid: %d, err: %v", cid, err)
	}
}

// receiveHandshake handles a handshake frame received from a client.
func (m *securityManager) receiveHandshake(conn network.Conn, buf buffer.Buffer) {
	payload, err := packet.ExtractHandshake(buf)
	if err != nil {
		buf.Release()
		log.Warnf("extract handshake frame failed, cid: %d, err: %v", conn.ID(), err)
		m.closeConn(conn.ID())
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
		m.closeConn(conn.ID())
		return true
	}

	state := value.(*securityState)

	session := state.session.Load()

	if session == nil {
		buf.Release()
		log.Warnf("session is not confirmed, cid: %d", conn.ID())
		m.closeConn(conn.ID())
		return true
	}

	output := buffer.MallocBytesWithHeadroom(packet.MessageBufferHeadroom(), max(1, len(payload)))
	plaintext, err := session.OpenMessage(output.Bytes()[:0], payload, handshake.ClientToServer, route, seq)
	buf.Release()

	if err != nil {
		output.Release()
		log.Warnf("open secure message failed, cid: %d, route: %d, seq: %d, err: %v", conn.ID(), route, seq, err)
		m.closeConn(conn.ID())
		return true
	}

	output.Truncate(len(plaintext))

	packed, err := packet.PackMessageBuffer(route, seq, output)
	if err != nil {
		output.Release()
		log.Errorf("pack message failed, cid: %d, err: %v", conn.ID(), err)
		return true
	}

	m.gate.proxy.deliver(m.gate.ctx, conn, route, seq, packed)

	return true
}

// advanceHandshake advances the handshake state machine of the given connection.
func (m *securityManager) advanceHandshake(conn network.Conn, frame []byte) {
	value, ok := m.states.Load(conn.ID())
	if !ok {
		m.closeConn(conn.ID())
		return
	}
	state := value.(*securityState)

	state.mu.Lock()

	if state.closed || state.session.Load() != nil {
		state.mu.Unlock()
		return
	}

	if state.pending == nil {
		pending, reply, err := m.server.Accept(frame)
		if err != nil {
			state.mu.Unlock()
			log.Warnf("accept handshake failed, cid: %d, err: %v", conn.ID(), err)
			m.closeConn(conn.ID())
			return
		}

		state.pending = pending
		state.mu.Unlock()
		m.pushFrame(conn, reply)

		return
	}

	session, err := m.server.Confirm(state.pending, frame)
	if err != nil {
		state.mu.Unlock()
		log.Warnf("confirm handshake failed, cid: %d, err: %v", conn.ID(), err)
		m.closeConn(conn.ID())
		return
	}

	state.pending = nil
	state.session.Store(session)
	if state.timer != nil {
		state.timer.Stop()
		state.timer = nil
	}
	m.gate.proxy.trigger(m.gate.ctx, cluster.Connect, conn.ID(), conn.UID())
	state.mu.Unlock()
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
func (m *securityManager) push(conn network.Conn, state *securityState, buf buffer.Buffer) error {
	session := state.session.Load()

	if session == nil {
		return errors.New("session is not confirmed")
	}

	route, seq, payload, err := packet.ExtractMessage(buf)
	if err != nil {
		return err
	}

	output := buffer.MallocBytesWithHeadroom(packet.MessageBufferHeadroom(), len(payload)+session.Overhead())

	if _, err = session.SealMessage(output.Bytes()[:0], payload, handshake.ServerToClient, route, seq); err != nil {
		output.Release()
		return err
	}

	packed, err := packet.PackMessageBuffer(route, seq, output)
	if err != nil {
		output.Release()
		return err
	}

	if err = conn.Push(packed); err != nil {
		packed.Release()
		return err
	}

	buf.Release()

	return nil
}
