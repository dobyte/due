package quic

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
	"github.com/quic-go/quic-go"
)

type serverConn struct {
	id                int64                       // Connection ID
	uid               atomic.Int64                // User ID
	attr              *attr                       // Connection attributes
	state             atomic.Int32                // Connection state
	connMgr           *serverConnMgr              // Connection manager
	rw                sync.RWMutex                // Lock
	wg1               *sync.WaitGroup             // Read wait group
	wg2               *sync.WaitGroup             // Write wait group
	qc                *quic.Conn                  // QUIC connection
	stream            *quic.Stream                // Bidirectional stream
	queue             *queue.Queue[buffer.Buffer] // Message queue
	output            *bufferWriter               // Write helper
	dueBuffers        []buffer.Buffer             // Buffers pending write
	lastHeartbeatTime atomic.Int64                // Time of the last heartbeat
	authorizeTimer    atomic.Value                // Authorize timer
}

var _ network.Conn = &serverConn{}

// ID returns the connection ID.
func (c *serverConn) ID() int64 {
	return c.id
}

// UID returns the user ID, or 0 when none is bound.
func (c *serverConn) UID() int64 {
	return c.uid.Load()
}

// Attr returns the attribute interface.
func (c *serverConn) Attr() network.Attr {
	return c.attr
}

// Bind binds uid to the connection.
//
// A successful bind cancels the authorize check timer.
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
// After unbinding, the authorize check timer is restarted.
func (c *serverConn) Unbind() error {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}

	c.uid.Store(0)
	c.checkAuthorize(c.stream)

	c.rw.RUnlock()

	return nil
}

// Push sends a message.
//
// The message is written to the send queue. Ownership of buf is transferred to the network layer
// only when Push returns nil; otherwise releasing the buffer is left to the caller.
func (c *serverConn) Push(buf buffer.Buffer) error {
	if buf == nil || buf.Len() == 0 {
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

// State returns the connection state.
func (c *serverConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close closes the connection.
//
// After the connection is closed its object is recycled and reused, so none of its methods or
// attributes may be used afterwards; the identity it carries may drift.
func (c *serverConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose()
	}
	return c.graceClose()
}

// LocalIP returns the local IP address.
func (c *serverConn) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr returns the local address.
func (c *serverConn) LocalAddr() (net.Addr, error) {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return nil, errors.ErrConnectionClosed
	}

	qc := c.qc
	c.rw.RUnlock()

	return qc.LocalAddr(), nil
}

// RemoteIP returns the remote IP address.
func (c *serverConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr returns the remote address.
func (c *serverConn) RemoteAddr() (net.Addr, error) {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return nil, errors.ErrConnectionClosed
	}

	qc := c.qc
	c.rw.RUnlock()

	return qc.RemoteAddr(), nil
}

// init initializes the connection.
//
// It reuses a connection object from the pool, resetting its state, linking it into a partition and
// starting its read and write goroutines under the write lock. The close path takes the same lock
// and is therefore blocked until initialization completes. When the manager has been closed the
// link is refused, initialization is aborted and the state is reset; the caller returns the
// connection object and closes the underlying connection, which prevents ghost connections from
// lingering after the server has stopped.
func (c *serverConn) init(id int64, qc *quic.Conn, stream *quic.Stream) bool {
	c.rw.Lock()

	c.id = id
	c.uid.Store(0)
	c.attr.values.Clear()
	c.state.Store(int32(network.ConnOpened))
	c.qc = qc
	c.stream = stream
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(minWriteQueueSize, c.connMgr.server.opts.writeQueueSize)), c.connMgr.server.opts.writeTimeout)
	c.output = newBufferWriter(stream)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.authorizeTimer.Store((*time.Timer)(nil))

	if !c.connMgr.linkConn(id, c) {
		// The manager has been closed or the reserved entry has already been cleaned up; reset
		// the state, clear the references and abort initialization.
		c.state.Store(int32(network.ConnClosed))
		c.qc = nil
		c.stream = nil
		c.queue = nil
		c.output = nil
		c.rw.Unlock()
		return false
	}

	c.wg1 = &sync.WaitGroup{}
	c.wg2 = &sync.WaitGroup{}

	c.wg2.Go(func() { c.write(stream) })
	c.wg1.Go(func() {
		// The connection may already have been closed concurrently before initialization
		// finished; run the authorize check and the connect hook only while the connection is
		// still opened.
		if c.State() == network.ConnOpened {
			c.checkAuthorize(stream)

			if c.connMgr.server.connectHandler != nil {
				c.connMgr.server.connectHandler(c)
			}
		}

		c.read(stream)
	})

	c.rw.Unlock()

	return true
}

// reset resets the connection.
//
// It clears the references and state inside the connection object so that it can be safely reused
// after being returned to the pool.
func (c *serverConn) reset() {
	c.wg1 = nil
	c.wg2 = nil
	c.queue = nil
	c.output = nil
	c.attr.values.Clear()
	c.dueBuffers = c.dueBuffers[:0]
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
// bound yet. The timer callback compares the connection ID and the stream pointer so that it does
// not close a new connection by mistake after the connection object has been recycled.
func (c *serverConn) checkAuthorize(stream *quic.Stream) {
	if c.connMgr.server.opts.authorizeTimeout <= 0 {
		return
	}

	id := c.id

	timer := c.authorizeTimer.Swap(time.AfterFunc(c.connMgr.server.opts.authorizeTimeout, func() {
		if c.UID() != 0 {
			return
		}

		c.recycleClose(stream, id)
	}))

	if t, ok := timer.(*time.Timer); ok && t != nil {
		t.Stop()
	}
}

// uncheckAuthorize cancels the authorization check.
//
// It stops and clears the authorize timeout timer, which lifts the authorize check when a user ID
// is bound or the connection is closed.
func (c *serverConn) uncheckAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout <= 0 {
		return
	}

	timer := c.authorizeTimer.Swap((*time.Timer)(nil))

	if t, ok := timer.(*time.Timer); ok && t != nil {
		t.Stop()
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
	if c.qc == nil {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}
	qc := c.qc
	q := c.queue
	err := q.Write(buffer.NewBytes(nil))
	c.rw.RUnlock()

	if err == nil {
		if closeTimeout := c.connMgr.server.opts.closeTimeout; closeTimeout > 0 {
			// A drain timeout disconnects the underlying connection, interrupting any write
			// operation that may be blocking in the write goroutine.
			timer := time.AfterFunc(closeTimeout, func() { _ = qc.CloseWithError(0, "close timeout") })
			q.Wait()
			timer.Stop()
		} else {
			q.Wait()
		}
	}

	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose(true)
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

	return c.doClose(false)
}

// recycleClose forcibly closes the connection if it is still the given one.
//
// The read and write goroutine error paths close the connection asynchronously through the task
// pool. By the time the closure runs, the connection object may already have been recycled, so both
// the connection ID and the stream pointer are compared to avoid closing a new connection by
// mistake.
func (c *serverConn) recycleClose(stream *quic.Stream, id int64) {
	c.rw.RLock()
	match := c.id == id && c.stream == stream
	c.rw.RUnlock()

	if match {
		c.forceClose()
	}
}

// doClose performs the close operation.
//
// It closes the write queue, waits for the read and write goroutines to exit, closes the stream and
// the QUIC connection, triggers the disconnect hook and returns the connection object to the pool.
// When graceful is true it waits for the write goroutine to drain and sends FIN first, then dwells
// for closeTimeout to let the peer acknowledge. When graceful is false it disconnects the QUIC
// connection directly to interrupt any write operation that may be blocking in the write goroutine,
// which guarantees forced-close semantics.
func (c *serverConn) doClose(graceful bool) error {
	c.rw.Lock()
	if c.qc == nil {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	c.queue.Close()
	qc := c.qc
	stream := c.stream
	c.qc = nil
	c.stream = nil
	c.rw.Unlock()

	if graceful {
		// Wait for the write goroutine to drain every accepted message before sending FIN.
		c.wg2.Wait()

		_ = stream.Close()

		// Keep the transport alive for closeTimeout so that the peer can acknowledge FIN and
		// retransmit, then tear it down.
		select {
		case <-qc.Context().Done():
		case <-time.After(c.connMgr.server.opts.closeTimeout):
		}
	}

	err := qc.CloseWithError(0, "closed")

	c.wg2.Wait()
	c.wg1.Wait()

	for buf := range c.queue.Read() {
		buf.Release()
	}

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	c.connMgr.recycleConn(c)

	return err
}

// read reads messages.
//
// It keeps reading messages from the stream, updates the heartbeat time, detects empty and
// heartbeat packets and dispatches them to the receive hook; a read failure triggers a
// recycle-based forced close.
func (c *serverConn) read(stream *quic.Stream) {
	var (
		index = 0
		id    = c.id
	)

	for {
		isHeartbeat, heartbeatTime, buf, err := packet.Read(stream)
		if err != nil {
			taskpool.Add(func() { c.recycleClose(stream, id) })
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
			if c.connMgr.server.opts.heartbeatInterval > 0 {
				c.lastHeartbeatTime.Store(time.Now().UnixNano())
			}

			if c.connMgr.server.opts.heartbeatMechanism == RespHeartbeat {
				hb := packet.PackHeartbeat(true)

				c.rw.RLock()
				err := c.queue.Write(hb)
				c.rw.RUnlock()

				if err != nil {
					hb.Release()
				}
			}

			if c.connMgr.server.heartbeatHandler != nil {
				c.connMgr.server.heartbeatHandler(c, heartbeatTime)
			}
		} else {
			if c.connMgr.server.opts.heartbeatInterval > 0 {
				index++
				if index%10 == 0 {
					c.lastHeartbeatTime.Store(time.Now().UnixNano())
				}
			}

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
// It takes messages from the write queue in a batch and writes them to the stream, and triggers
// heartbeat detection and dispatch at the heartbeat interval.
func (c *serverConn) write(stream *quic.Stream) {
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

			c.doBatchWrite(stream, buf)
		case t, ok := <-tickerC:
			if !ok {
				return
			}

			if !c.doHandleHeartbeat(stream, t) {
				return
			}
		}
	}
}

// doBatchWrite writes messages in a batch.
//
// It takes tasks from the write queue in a batch and writes them one by one to reduce the number of
// queue channel operations. Writing to a QUIC stream copies in user space, so unlike TCP there is
// no need to aggregate byte slices and dispatch them in one call.
func (c *serverConn) doBatchWrite(stream *quic.Stream, first buffer.Buffer) {
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
	for _, buf := range c.dueBuffers {
		if c.connMgr.server.opts.writeTimeout > 0 {
			_ = stream.SetWriteDeadline(time.Now().Add(c.connMgr.server.opts.writeTimeout))
		}

		if err := c.output.write(buf); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Warnf("write message error: %v", err)

			id := c.id

			taskpool.Add(func() { c.recycleClose(stream, id) })
		}

		buf.Release()
	}

	c.dueBuffers = c.dueBuffers[:0]
}

// doHandleHeartbeat handles heartbeats.
//
// It checks whether the time of the last received message has timed out and triggers a
// recycle-based forced close when it has; in active periodic heartbeat mode it also dispatches a
// heartbeat packet. It returns whether to keep the write goroutine looping, which is false on a
// heartbeat timeout.
func (c *serverConn) doHandleHeartbeat(stream *quic.Stream, t time.Time) bool {
	if c.lastHeartbeatTime.Load() < t.Add(-2*c.connMgr.server.opts.heartbeatInterval).UnixNano() {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		id := c.id

		taskpool.Add(func() { c.recycleClose(stream, id) })

		return false
	}

	if c.connMgr.server.opts.heartbeatMechanism == TickHeartbeat {
		if c.connMgr.server.opts.writeTimeout > 0 {
			_ = stream.SetWriteDeadline(time.Now().Add(c.connMgr.server.opts.writeTimeout))
		}

		hb := packet.PackHeartbeat(true)

		if err := c.output.write(hb); err != nil {
			log.Warnf("write heartbeat message error: %v", err)
		}

		hb.Release()
	}

	return true
}

// isClosed reports whether the connection state is closed.
func (c *serverConn) isClosed() bool {
	return c.State() == network.ConnClosed
}
