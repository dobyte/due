package ws

import (
	"io"
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
	"github.com/gorilla/websocket"
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
	conn              *websocket.Conn             // Underlying WS connection
	remoteAddr        net.Addr                    // Real client address (resolved from the proxy headers in the application proxy mode)
	queue             *queue.Queue[buffer.Buffer] // Message queue
	lastHeartbeatTime atomic.Int64                // Time of the last received heartbeat
	authorizeTimer    atomic.Pointer[time.Timer]  // Authorization timer
}

var _ network.Conn = &serverConn{}

// ID returns the connection ID.
func (c *serverConn) ID() int64 {
	return c.id
}

// UID returns the user ID.
func (c *serverConn) UID() int64 {
	return c.uid.Load()
}

// Attr returns the attribute interface of the connection.
func (c *serverConn) Attr() network.Attr {
	return c.attr
}

// Bind binds the connection to the given user ID.
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

// Unbind unbinds the user ID from the connection.
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
// The caller controls when buf is released when the send fails.
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

// State returns the connection state.
func (c *serverConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close closes the connection.
//
// After the connection is closed the connection object is recycled and reused, so none of its
// methods or attributes may be used again (the identity may drift). When force is true the
// connection is closed forcibly.
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

// LocalAddr returns the local address.
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

// RemoteAddr returns the remote address.
//
// It returns the real client address resolved from the proxy headers when available, and the
// remote address of the underlying connection otherwise.
func (c *serverConn) RemoteAddr() (net.Addr, error) {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return nil, errors.ErrConnectionClosed
	}

	conn := c.conn
	remoteAddr := c.remoteAddr
	c.rw.RUnlock()

	if remoteAddr != nil {
		return remoteAddr, nil
	}

	return conn.RemoteAddr(), nil
}

// init initializes the connection.
//
// It reuses a connection object from the object pool and, under the write lock, resets the state,
// stores the connection in a partition and starts the read and write goroutines. The close path and
// the delayed close tasks of the previous life cycle both need the lock, so they block until
// initialization completes, which prevents the connection object from being recycled before the
// goroutines start. When a partition refuses the store while the server is closing, initialization
// aborts and the caller returns the connection object to the object pool and closes the underlying
// connection. remoteAddr is the real client address and may be nil.
func (c *serverConn) init(conn *websocket.Conn, remoteAddr net.Addr) bool {
	c.rw.Lock()

	c.id = c.connMgr.cid.Add(1)
	c.uid.Store(0)
	c.attr.values.Clear()
	c.state.Store(int32(network.ConnOpened))
	c.conn = conn
	c.remoteAddr = remoteAddr
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(minWriteQueueSize, c.connMgr.server.opts.writeQueueSize)), c.connMgr.server.opts.writeTimeout)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.authorizeTimer.Store(nil)

	if !c.connMgr.storeConn(conn, c) {
		// The partition has stopped accepting connections, so reset the state and clear the references before aborting initialization.
		c.state.Store(int32(network.ConnClosed))
		c.conn = nil
		c.remoteAddr = nil
		c.queue = nil
		c.rw.Unlock()
		return false
	}

	c.wg1.Go(func() { c.read(conn) })
	c.wg2.Go(func() { c.write(conn) })

	c.rw.Unlock()

	// The connection may have been closed concurrently before initialization completes, so run the authorization check and the connect hook only while it is still opened.
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
// It clears the references and state held by the connection object so that it can be safely reused
// after being returned to the object pool, and explicitly stops the remaining authorization timer
// so that its callback does not fire after the connection is reused.
func (c *serverConn) reset() {
	c.remoteAddr = nil
	c.queue = nil
	c.attr.values.Clear()
	c.uncheckAuthorize()
}

// checkState checks the connection state. It returns the matching error for the hanged or closed
// state, and nil when the connection is normal.
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

// checkAuthorize performs the authorization check.
//
// It starts the authorization timeout timer and closes the connection when the timeout elapses
// while no user ID has been bound. The timer callback compares both the connection ID and the
// connection pointer so that a recycled connection is not closed by mistake.
func (c *serverConn) checkAuthorize(conn *websocket.Conn) {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		id := c.id

		if timer := c.authorizeTimer.Swap(time.AfterFunc(c.connMgr.server.opts.authorizeTimeout, func() {
			if c.UID() != 0 {
				return
			}

			c.recycleClose(conn, id)
		})); timer != nil {
			timer.Stop()
		}
	}
}

// uncheckAuthorize cancels the authorization check.
//
// It stops the authorization timeout timer and is used to end the authorization check when a user
// ID is bound or the connection is closed.
func (c *serverConn) uncheckAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		if timer := c.authorizeTimer.Swap(nil); timer != nil {
			timer.Stop()
		}
	}
}

// graceClose closes the connection gracefully.
//
// It writes a close signal and waits for the write queue to drain before closing the connection, so
// that buffered messages can be delivered as far as possible. When a graceful close timeout is
// configured, the underlying connection is closed once the timeout elapses before the queue drains,
// which forces the wait to end.
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
			// Close the underlying connection once the drain times out to interrupt a possibly blocking write in the write goroutine.
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
// It immediately switches the state to closed and closes the connection without waiting for the
// write queue to drain.
func (c *serverConn) forceClose() error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	c.uncheckAuthorize()

	return c.doClose(true)
}

// doClose performs the close.
//
// It closes the write queue, waits for the read and write goroutines to exit, closes the WS
// connection, invokes the disconnect handler and returns the connection object to the connection
// pool. When force is true it closes the WS connection first so that a possibly blocking write in
// the write goroutine is interrupted, preserving the force-close semantics.
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

// recycleClose forcibly closes the connection when it is still the given connection.
//
// The read and write goroutines close the connection asynchronously through the task pool on their
// error paths. By the time the closure runs the connection object may have been recycled and the
// address of the WS connection may have been reused by the runtime, so both the connection ID and
// the WS connection pointer must match to avoid closing a new connection by mistake.
func (c *serverConn) recycleClose(conn *websocket.Conn, id int64) {
	c.rw.RLock()
	match := c.id == id && c.conn == conn
	c.rw.RUnlock()

	if match {
		c.forceClose()
	}
}

// read reads messages.
//
// It keeps reading messages, updates the heartbeat time, detects empty and heartbeat packets and
// dispatches them to the receive handler. A read failure triggers a force close.
func (c *serverConn) read(conn *websocket.Conn) {
	var (
		index = 0
		id    = c.id
	)

	for {
		mt, r, err := conn.NextReader()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				if _, ok := err.(*websocket.CloseError); !ok {
					log.Warnf("read message failed: %d %v", c.id, err)
				}
			}

			taskpool.Add(func() { c.recycleClose(conn, id) })
			return
		}

		if mt != websocket.BinaryMessage {
			_, _ = io.Copy(io.Discard, r)
			continue
		}

		isHeartbeat, heartbeatTime, buf, err := packet.Read(r)
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

			// trigger heartbeat handler, heartbeat time is not nil
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
func (c *serverConn) write(conn *websocket.Conn) {
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

			c.doWrite(conn, buf)
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

// doWrite performs a write.
//
// It terminates the write goroutine after recognizing the close signal, and otherwise writes the
// message data and releases the buffer.
func (c *serverConn) doWrite(conn *websocket.Conn, buf buffer.Buffer) {
	closeSig := buf.Len() == 0

	c.queue.Done(closeSig)

	if closeSig {
		buf.Release()
		return
	}

	if c.connMgr.server.opts.writeTimeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(c.connMgr.server.opts.writeTimeout))
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, buf.Bytes()); err != nil {
		if _, ok := err.(*websocket.CloseError); !ok {
			if !errors.Is(err, net.ErrClosed) {
				log.Warnf("write message error: %v", err)

				id := c.id

				taskpool.Add(func() { c.recycleClose(conn, id) })
			}
		}
	}

	buf.Release()
}

// doHandleHeartbeat handles heartbeats.
//
// It checks whether the time of the last received message has timed out and triggers a force close
// if so; in the active tick heartbeat mode it also sends a heartbeat packet. It returns false when
// the write goroutine should stop because the heartbeat timed out.
func (c *serverConn) doHandleHeartbeat(conn *websocket.Conn, t time.Time) bool {
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

			if err := conn.WriteMessage(websocket.BinaryMessage, hb.Bytes()); err != nil {
				log.Warnf("write heartbeat message error: %v", err)
			}

			hb.Release()
		}

		return true
	}
}

// isClosed reports whether the connection state is closed.
func (c *serverConn) isClosed() bool {
	return c.State() == network.ConnClosed
}
