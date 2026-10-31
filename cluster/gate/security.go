package gate

import (
	"context"
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
	gate    *Gate
	server  *handshake.Server
	timeout time.Duration
	// trigger delivers the connection lifecycle events to the business nodes.
	trigger func(ctx context.Context, event cluster.Event, cid, uid int64)

	states   sync.Map       // connection ID -> *securityState
	conns    sync.Map       // connection ID -> *securedConn
	pending  sync.Map       // connection ID -> []byte; handshake frame received before admission
	closerMu sync.Mutex     // serializes the closer registration with shutdown
	draining bool           // protected by closerMu; reports that new closes are no longer accepted
	closers  sync.WaitGroup // security close goroutines, joined by shutdown
}

// securityState is the security state of a single connection.
type securityState struct {
	mu        sync.Mutex
	pending   *handshake.Pending
	session   atomic.Pointer[handshake.Session]
	closed    bool        // protected by mu
	closing   bool        // protected by mu; at most one security close goroutine per state
	connected bool        // protected by mu; the Connect event has been committed for delivery
	timer     *time.Timer // protected by mu
	eventMu   sync.Mutex  // serializes the Connect and Disconnect event deliveries of the connection
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
	m := &securityManager{
		gate:    gate,
		server:  handshake.NewServer(opts.signer, opts.suites...),
		timeout: timeout,
	}
	m.trigger = func(ctx context.Context, event cluster.Event, cid, uid int64) {
		m.gate.proxy.trigger(ctx, event, cid, uid)
	}

	return m
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

// remove releases the state of the given connection and returns it.
//
// The state is handed to [securityManager.deliverDisconnect] by the disconnect path so that a
// Disconnect event cannot overtake the Connect event of the same connection. The state is marked
// closed here, which also turns a Connect delivery that has not been committed yet into a no-op.
func (m *securityManager) remove(cid int64) *securityState {
	m.pending.Delete(cid)

	var state *securityState

	if value, ok := m.states.LoadAndDelete(cid); ok {
		state = value.(*securityState)
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

	return state
}

// triggerConnect delivers the Connect event of a confirmed handshake.
//
// The delivery is serialized with [securityManager.deliverDisconnect] through the event lock of the
// connection state. A disconnect racing the confirmation either waits for this delivery or marks the
// state closed first, which turns this call into a no-op, so the Connect and Disconnect events always
// reach the business node in order and a connection removed before its Connect event was committed
// produces no Disconnect event.
func (m *securityManager) triggerConnect(conn network.Conn) {
	cid := conn.ID()

	value, ok := m.states.Load(cid)
	if !ok {
		return
	}

	state := value.(*securityState)

	state.eventMu.Lock()

	state.mu.Lock()
	if state.closed || state.connected {
		state.mu.Unlock()
		state.eventMu.Unlock()
		return
	}
	state.connected = true
	state.mu.Unlock()

	// The event handler may perform an RPC, so the state lock is already released; the event lock
	// keeps a concurrent Disconnect delivery waiting until this delivery finishes.
	m.trigger(m.gate.ctx, cluster.Connect, cid, conn.UID())

	state.eventMu.Unlock()
}

// deliverDisconnect delivers the Disconnect event of a removed connection when its paired Connect
// event was committed for delivery.
//
// It takes the event lock, so a Connect delivery that is already in flight completes first. The state
// has already been removed from the manager and marked closed, so no Connect can be committed after
// the lock is acquired.
func (m *securityManager) deliverDisconnect(state *securityState, cid, uid int64) {
	if state == nil {
		return
	}

	state.eventMu.Lock()
	state.mu.Lock()
	connected := state.connected
	state.mu.Unlock()
	state.eventMu.Unlock()

	if !connected {
		return
	}

	m.trigger(m.gate.ctx, cluster.Disconnect, cid, uid)
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

// closeSecureConn forcibly closes a connection that is still the one recorded for cid.
//
// The compare-and-close runs inside the transport, so a connection object that has already been
// recycled and reused by a newer connection is not closed by mistake.
func (m *securityManager) closeSecureConn(cid int64, conn *securedConn) {
	value, ok := m.states.Load(cid)
	if !ok || value != conn.state {
		return
	}
	if current, ok := m.conns.Load(cid); !ok || current != conn {
		return
	}
	if err := conn.Conn.CloseIf(cid); err != nil && !errors.Is(err, errors.ErrConnectionClosed) {
		log.Warnf("close secure connection failed, cid: %d, err: %v", cid, err)
	}
}

// handleHandshake handles a handshake frame received from a client.
func (m *securityManager) handleHandshake(conn network.Conn, buf buffer.Buffer) {
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
//
// A handshake frame received before the connection has been admitted is buffered and processed by
// [securityManager.acceptPending] once the gate registers the session; a frame received after the
// session is confirmed is a protocol violation and closes the connection, so the handshake path
// cannot be used to bypass the connection rate limiter.
func (m *securityManager) advanceHandshake(conn network.Conn, frame []byte) {
	value, ok := m.states.Load(conn.ID())
	if !ok {
		m.deferHandshake(conn, frame)
		return
	}
	state := value.(*securityState)

	state.mu.Lock()

	if state.closed {
		state.mu.Unlock()
		return
	}

	if state.session.Load() != nil {
		state.mu.Unlock()
		log.Warnf("unexpected handshake frame after the session is confirmed, cid: %d", conn.ID())
		m.closeConn(conn.ID())
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
	state.mu.Unlock()

	// Trigger the event after releasing the lock: the event handler may perform an RPC, and holding
	// the state lock would block the read loop and the handshake timer for its whole duration. The
	// delivery is still serialized with a concurrent Disconnect event by the connection event lock.
	m.triggerConnect(conn)
}

// deferHandshake keeps a handshake frame that arrived before the connection was admitted.
//
// Some transports start the read loop before invoking the connect callback, so the first
// ClientHello can race the gate admission. The frame is processed by
// [securityManager.acceptPending] after the session is registered, which keeps the handshake and
// the Connect event in the order the rest of the gate assumes. A frame for a connection that is
// already closing is dropped.
func (m *securityManager) deferHandshake(conn network.Conn, frame []byte) {
	if len(frame) == 0 || conn.State() != network.ConnOpened {
		return
	}

	m.pending.LoadOrStore(conn.ID(), frame)
}

// acceptPending processes the handshake frame buffered before the connection was admitted.
func (m *securityManager) acceptPending(conn network.Conn) {
	value, ok := m.pending.LoadAndDelete(conn.ID())
	if !ok {
		return
	}

	m.advanceHandshake(conn, value.([]byte))
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
