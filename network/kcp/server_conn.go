package kcp

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	taskpool "github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/utils/xnet"
	"github.com/xtaci/kcp-go/v5"
)

type serverConn struct {
	id                int64                       // Connection ID
	uid               atomic.Int64                // User ID
	attr              *attr                       // Connection attributes
	state             atomic.Int32                // Connection state
	connMgr           *serverConnMgr              // Connection manager
	rw                sync.RWMutex                // Lock
	wg1               sync.WaitGroup              // Read wait group, reused with the connection object pool
	wg2               sync.WaitGroup              // Write wait group, reused with the connection object pool
	conn              *kcp.UDPSession             // Source KCP connection
	queue             *queue.Queue[buffer.Buffer] // Message queue
	dueBuffers        []buffer.Buffer             // Buffers pending write
	netBuffers        net.Buffers                 // Byte slices pending write
	lastHeartbeatTime atomic.Int64                // Time of the last heartbeat
	authorizeTimer    atomic.Value                // Authorize timer
}

var _ network.Conn = &serverConn{}

// ID returns the connection ID.
func (c *serverConn) ID() int64 {
	return c.id
}

// UID returns the bound user ID, or 0 when none is bound.
func (c *serverConn) UID() int64 {
	return c.uid.Load()
}

// Attr returns the attribute interface used to read and write custom attributes.
func (c *serverConn) Attr() network.Attr {
	return c.attr
}

// Bind binds uid to the connection.
//
// A successful bind cancels the periodic authorize check. It reports
// [errors.ErrConnectionClosed] when the connection has already been closed.
func (c *serverConn) Bind(uid int64) error {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}

	c.uid.Store(uid)
	c.uncheckAuthorize()

	c.rw.RUnlock()

	return nil
}

// Unbind removes the bound user ID.
//
// After unbinding, the periodic authorize check is restarted. It reports
// [errors.ErrConnectionClosed] when the connection has already been closed.
func (c *serverConn) Unbind() error {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}

	c.uid.Store(0)
	c.checkAuthorize(c.conn)

	c.rw.RUnlock()

	return nil
}

// Push sends a message.
//
// The message is written to the write queue and dispatched by the write goroutine. When the send
// fails, releasing the buffer is left to the caller.
func (c *serverConn) Push(buf buffer.Buffer) error {
	if buf.Len() == 0 {
		return errors.ErrInvalidMessage
	}

	c.rw.RLock()

	if err := c.checkState(); err != nil {
		c.rw.RUnlock()
		return err
	}

	err := c.queue.Write(buf)

	c.rw.RUnlock()

	return err
}

// State returns the current connection state.
func (c *serverConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close closes the connection.
//
// After the connection is closed its object is recycled and reused, so none of its methods or
// attributes may be used afterwards; the identity it carries may drift. It closes immediately when
// force is true; otherwise it performs a graceful close.
func (c *serverConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose()
	} else {
		return c.graceClose()
	}
}

// LocalIP returns the local IP address.
func (c *serverConn) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr returns the local network address.
func (c *serverConn) LocalAddr() (net.Addr, error) {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return nil, errors.ErrConnectionClosed
	}

	conn := c.conn
	c.rw.RUnlock()

	return conn.LocalAddr(), nil
}

// RemoteIP returns the remote IP address.
func (c *serverConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr returns the remote network address.
func (c *serverConn) RemoteAddr() (net.Addr, error) {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return nil, errors.ErrConnectionClosed
	}

	conn := c.conn
	c.rw.RUnlock()

	return conn.RemoteAddr(), nil
}

// init initializes the connection.
//
// It reuses a connection object from the pool, resetting its state, storing it in a partition and
// starting its read and write goroutines under the write lock. The close path and the delayed close
// task of the previous lifecycle both take the same lock, so they are blocked until initialization
// completes, preventing the connection object from being recycled before its goroutines start. If a
// partition refuses storage while the server is shutting down, initialization is aborted and the
// caller returns the connection object and closes the underlying connection. It reports whether
// initialization succeeded, which is false while the server is shutting down.
func (c *serverConn) init(conn *kcp.UDPSession) bool {
	c.rw.Lock()

	c.id = c.connMgr.cid.Add(1)
	c.uid.Store(0)
	c.attr.values.Clear()
	c.state.Store(int32(network.ConnOpened))
	c.conn = conn
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(minWriteQueueSize, c.connMgr.server.opts.writeQueueSize)), c.connMgr.server.opts.writeTimeout)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.authorizeTimer.Store((*time.Timer)(nil))

	if c.connMgr.server.opts.mtu > 0 {
		conn.SetMtu(c.connMgr.server.opts.mtu)
	}

	if len(c.connMgr.server.opts.noDelay) == 4 {
		conn.SetNoDelay(c.connMgr.server.opts.noDelay[0], c.connMgr.server.opts.noDelay[1], c.connMgr.server.opts.noDelay[2], c.connMgr.server.opts.noDelay[3])
	}

	if c.connMgr.server.opts.ackNoDelay {
		conn.SetACKNoDelay(c.connMgr.server.opts.ackNoDelay)
	}

	if c.connMgr.server.opts.writeDelay {
		conn.SetWriteDelay(c.connMgr.server.opts.writeDelay)
	}

	if len(c.connMgr.server.opts.windowSize) == 2 {
		conn.SetWindowSize(c.connMgr.server.opts.windowSize[0], c.connMgr.server.opts.windowSize[1])
	}

	if c.connMgr.server.opts.readBuffer > 0 {
		conn.SetReadBuffer(c.connMgr.server.opts.readBuffer)
	}

	if c.connMgr.server.opts.writeBuffer > 0 {
		conn.SetWriteBuffer(c.connMgr.server.opts.writeBuffer)
	}

	if !c.connMgr.storeConn(conn, c) {
		// The partition has stopped accepting connections; reset the state, clear the references
		// and abort initialization.
		c.state.Store(int32(network.ConnClosed))
		c.conn = nil
		c.queue = nil
		c.rw.Unlock()
		return false
	}

	c.wg1.Go(func() { c.read(conn) })
	c.wg2.Go(func() { c.write(conn) })

	c.rw.Unlock()

	// The connection may already have been closed concurrently before initialization finished;
	// run the authorize check and the connect hook only while the connection is still opened.
	if c.State() != network.ConnOpened {
		return true
	}

	c.checkAuthorize(conn)

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}

	return true
}

// reset resets the connection.
//
// It clears the connection fields and attributes so that the object can be reused.
func (c *serverConn) reset() {
	c.queue = nil
	c.attr.values.Clear()
	c.dueBuffers = c.dueBuffers[:0]
	c.netBuffers = c.netBuffers[:0]
	c.uncheckAuthorize()
}

// checkState checks the connection state.
//
// It returns the error matching the hanged or closed state, and nil when the connection is normal:
// [errors.ErrConnectionHanged] when hanged and [errors.ErrConnectionClosed] when closed.
func (c *serverConn) checkState() error {
	switch c.State() {
	case network.ConnHanged:
		return errors.ErrConnectionHanged
	case network.ConnClosed:
		return errors.ErrConnectionClosed
	default:
		return nil
	}
}

// checkAuthorize checks authorization.
//
// It starts the authorize timeout timer, which closes the connection when it fires with no user ID
// bound yet. The timer callback compares the connection ID and the connection pointer so that it
// does not close a new connection by mistake after the connection object has been recycled.
func (c *serverConn) checkAuthorize(conn *kcp.UDPSession) {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		id := c.id

		timer := c.authorizeTimer.Swap(time.AfterFunc(c.connMgr.server.opts.authorizeTimeout, func() {
			if c.UID() != 0 {
				return
			}

			c.recycleClose(conn, id)
		}))
		if t, ok := timer.(*time.Timer); ok && t != nil {
			t.Stop()
		}
	}
}

// uncheckAuthorize cancels the authorization check.
//
// It stops and clears the authorize timer, lifting the constraint that forces a close.
func (c *serverConn) uncheckAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		timer := c.authorizeTimer.Swap((*time.Timer)(nil))

		if t, ok := timer.(*time.Timer); ok && t != nil {
			t.Stop()
		}
	}
}

// graceClose closes the connection gracefully.
//
// It writes the close signal, waits for the write queue to drain and then closes the connection,
// so that as many buffered messages as possible are delivered. When a graceful close timeout is
// configured, a queue that has not drained in time disconnects the underlying connection directly
// to end the wait forcibly.
func (c *serverConn) graceClose() error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnHanged)) {
		return errors.ErrConnectionNotOpened
	}

	c.uncheckAuthorize()

	c.rw.RLock()
	if c.conn == nil {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}
	conn := c.conn
	q := c.queue
	err := q.Write(buffer.NewBytes(nil))
	c.rw.RUnlock()

	if err == nil {
		if closeTimeout := c.connMgr.server.opts.closeTimeout; closeTimeout > 0 {
			// A drain timeout disconnects the underlying connection, interrupting any write
			// operation that may be blocking in the write goroutine.
			timer := time.AfterFunc(closeTimeout, func() { _ = conn.Close() })
			q.Wait()
			timer.Stop()
		} else {
			q.Wait()
		}
	}

	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose()
}

// forceClose closes the connection forcibly.
//
// It switches the state to closed and closes the connection immediately without waiting for the
// write queue to drain.
func (c *serverConn) forceClose() error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	c.uncheckAuthorize()

	return c.doClose(true)
}

// recycleClose forcibly closes the connection if it is still the given one.
//
// The read and write goroutine error paths close the connection asynchronously through the task
// pool. By the time the closure runs, the connection object may already have been recycled and the
// KCP connection object address may have been reused by the runtime, so both the connection ID and
// the KCP connection pointer are compared to avoid closing a new connection by mistake.
func (c *serverConn) recycleClose(conn *kcp.UDPSession, id int64) {
	c.rw.RLock()
	match := c.id == id && c.conn == conn
	c.rw.RUnlock()

	if match {
		c.forceClose()
	}
}

// doClose performs the close operation.
//
// It closes the write queue, waits for the read and write goroutines to exit, closes the KCP
// connection, triggers the disconnect hook and returns the connection object to the pool. When
// force is true it closes the KCP connection first to interrupt any write operation that may be
// blocking in the write goroutine, which guarantees forced-close semantics.
func (c *serverConn) doClose(force ...bool) error {
	c.rw.Lock()
	if c.conn == nil {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	c.queue.Close()
	conn := c.conn
	c.conn = nil
	c.rw.Unlock()

	var err error

	if len(force) > 0 && force[0] {
		err = conn.Close()
		c.wg2.Wait()
	} else {
		c.wg2.Wait()
		err = conn.Close()
	}

	c.wg1.Wait()

	for buf := range c.queue.Read() {
		buf.Release()
	}

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	c.connMgr.recycleConn(conn)

	return err
}

// read reads messages.
//
// It reads KCP data in a loop, checks the connection state and heartbeat packets, and either
// responds according to the heartbeat mechanism or hands valid messages to the receive hook.
func (c *serverConn) read(conn *kcp.UDPSession) {
	var (
		index = 0
		id    = c.id
	)

	for {
		isHeartbeat, heartbeatTime, buf, err := packet.Read(conn)
		if err != nil {
			taskpool.Add(func() { c.recycleClose(conn, id) })
			return
		}

		switch c.State() {
		case network.ConnClosed:
			if !isHeartbeat {
				buf.Release()
			}
			return
		case network.ConnHanged:
			if !isHeartbeat {
				buf.Release()
				return
			}
		}

		if isHeartbeat {
			// update heartbeat time
			if c.connMgr.server.opts.heartbeatInterval > 0 {
				c.lastHeartbeatTime.Store(time.Now().UnixNano())
			}

			// responsive heartbeat
			if c.connMgr.server.opts.heartbeatMechanism == RespHeartbeat {
				hb := packet.PackHeartbeat(true)

				c.rw.RLock()
				err := c.queue.Write(hb)
				c.rw.RUnlock()

				if err != nil {
					hb.Release()
				}
			}

			// trigger heartbeat handler
			if c.connMgr.server.heartbeatHandler != nil {
				c.connMgr.server.heartbeatHandler(c, heartbeatTime)
			}
		} else {
			// update heartbeat time
			if c.connMgr.server.opts.heartbeatInterval > 0 {
				index++

				if index%10 == 0 {
					c.lastHeartbeatTime.Store(time.Now().UnixNano())
				}
			}

			// ignore empty packet
			if buf.Len() == 0 {
				buf.Release()
				continue
			}

			if c.connMgr.server.receiveHandler != nil {
				c.connMgr.server.receiveHandler(c, buf)
			} else {
				buf.Release()
			}
		}
	}
}

// write writes messages.
//
// It takes messages from the write queue and writes them to the connection, and triggers heartbeat
// detection and dispatch at the heartbeat interval.
func (c *serverConn) write(conn *kcp.UDPSession) {
	var tickerC <-chan time.Time

	if c.connMgr.server.opts.heartbeatInterval > 0 {
		ticker := time.NewTicker(c.connMgr.server.opts.heartbeatInterval)
		defer ticker.Stop()
		tickerC = ticker.C
	}

	for {
		select {
		case buf, ok := <-c.queue.Read():
			if !ok {
				return
			}

			c.doBatchWrite(conn, buf)
		case t, ok := <-tickerC:
			if !ok {
				return
			}

			if !c.doHandleHeartbeat(conn, t) {
				return
			}
		}
	}
}

// doBatchWrite writes messages in a batch.
//
// It takes tasks from the write queue in a batch, collects their byte slices and dispatches them
// with a single WriteBuffers call to reduce the number of system calls.
func (c *serverConn) doBatchWrite(conn *kcp.UDPSession, first buffer.Buffer) {
	closeSig := first.Len() == 0

	c.queue.Done(closeSig)

	if closeSig {
		first.Release()
		return
	}

	c.dueBuffers = c.dueBuffers[:0]
	c.dueBuffers = append(c.dueBuffers, first)

	for len(c.dueBuffers) < maxBatchWriteNum {
		select {
		case buf, ok := <-c.queue.Read():
			if !ok {
				goto OVER
			}

			closeSig = buf.Len() == 0

			c.queue.Done(closeSig)

			if closeSig {
				buf.Release()
				goto OVER
			}

			c.dueBuffers = append(c.dueBuffers, buf)
		default:
			goto OVER
		}
	}

OVER:
	c.netBuffers = c.netBuffers[:0]

	for _, buf := range c.dueBuffers {
		buf.VisitBytes(func(bytes []byte) bool {
			c.netBuffers = append(c.netBuffers, bytes)
			return true
		})
	}

	if len(c.netBuffers) > 0 {
		if c.connMgr.server.opts.writeTimeout > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(c.connMgr.server.opts.writeTimeout))
		}

		if _, err := conn.WriteBuffers(c.netBuffers); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Warnf("write message error: %v", err)

				id := c.id

				taskpool.Add(func() { c.recycleClose(conn, id) })
			}
		}
	}

	for _, buf := range c.dueBuffers {
		buf.Release()
	}

	c.netBuffers = c.netBuffers[:0]
	c.dueBuffers = c.dueBuffers[:0]
}

// doHandleHeartbeat handles heartbeats.
//
// It forcibly closes the connection when the heartbeat timeout threshold is exceeded; otherwise it
// actively sends a heartbeat packet according to the heartbeat mechanism. It returns whether to
// keep running, which is false when the connection was forcibly closed for a heartbeat timeout.
func (c *serverConn) doHandleHeartbeat(conn *kcp.UDPSession, t time.Time) bool {
	if c.lastHeartbeatTime.Load() < t.Add(-2*c.connMgr.server.opts.heartbeatInterval).UnixNano() {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		id := c.id

		taskpool.Add(func() { c.recycleClose(conn, id) })

		return false
	} else {
		if c.connMgr.server.opts.heartbeatMechanism == TickHeartbeat {
			if c.connMgr.server.opts.writeTimeout > 0 {
				_ = conn.SetWriteDeadline(time.Now().Add(c.connMgr.server.opts.writeTimeout))
			}

			hb := packet.PackHeartbeat(true)

			if _, err := conn.Write(hb.Bytes()); err != nil {
				log.Warnf("write heartbeat message error: %v", err)
			}

			hb.Release()
		}
	}

	return true
}

// isClosed reports whether the connection state is closed.
func (c *serverConn) isClosed() bool {
	return c.State() == network.ConnClosed
}
