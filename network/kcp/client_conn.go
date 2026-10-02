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

type clientConn struct {
	rw                sync.RWMutex                // Lock
	id                int64                       // Connection ID
	uid               atomic.Int64                // User ID
	attr              *attr                       // Connection attributes
	conn              *kcp.UDPSession             // Source UDP connection
	state             atomic.Int32                // Connection state
	cli               *client                     // Client
	wg1               sync.WaitGroup              // Read wait group
	wg2               sync.WaitGroup              // Write wait group
	queue             *queue.Queue[buffer.Buffer] // Message queue
	dueBuffers        []buffer.Buffer             // Buffers pending write
	netBuffers        net.Buffers                 // Byte slices pending write
	lastHeartbeatTime atomic.Int64                // Time of the last heartbeat
}

var _ network.Conn = &clientConn{}

// newClientConn returns a new client connection.
//
// It initializes the connection state, the write queue and the two read/write goroutines, and
// applies the KCP parameters configured on the client.
func newClientConn(cli *client, conn *kcp.UDPSession) network.Conn {
	c := &clientConn{}
	c.id = cli.cid.Add(1)
	c.attr = &attr{}
	c.conn = conn
	c.cli = cli
	c.state.Store(int32(network.ConnOpened))
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(minWriteQueueSize, cli.opts.writeQueueSize)), cli.opts.writeTimeout)
	c.dueBuffers = make([]buffer.Buffer, 0, maxBatchWriteNum)
	c.netBuffers = make(net.Buffers, 0, 2*maxBatchWriteNum)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.wg1.Go(func() { c.read(conn) })
	c.wg2.Go(func() { c.write(conn) })

	if c.cli.opts.mtu > 0 {
		conn.SetMtu(c.cli.opts.mtu)
	}

	if len(c.cli.opts.noDelay) == 4 {
		conn.SetNoDelay(c.cli.opts.noDelay[0], c.cli.opts.noDelay[1], c.cli.opts.noDelay[2], c.cli.opts.noDelay[3])
	}

	if c.cli.opts.ackNoDelay {
		conn.SetACKNoDelay(c.cli.opts.ackNoDelay)
	}

	if c.cli.opts.writeDelay {
		conn.SetWriteDelay(c.cli.opts.writeDelay)
	}

	if len(c.cli.opts.windowSize) == 2 {
		conn.SetWindowSize(c.cli.opts.windowSize[0], c.cli.opts.windowSize[1])
	}

	if c.cli.opts.readBuffer > 0 {
		conn.SetReadBuffer(c.cli.opts.readBuffer)
	}

	if c.cli.opts.writeBuffer > 0 {
		conn.SetWriteBuffer(c.cli.opts.writeBuffer)
	}

	if c.cli.connectHandler != nil {
		c.cli.connectHandler(c)
	}

	return c
}

// ID returns the connection ID.
func (c *clientConn) ID() int64 {
	return c.id
}

// UID returns the bound user ID, or 0 when none is bound.
func (c *clientConn) UID() int64 {
	return c.uid.Load()
}

// Attr returns the attribute interface used to read and write custom attributes.
func (c *clientConn) Attr() network.Attr {
	return c.attr
}

// Bind binds uid to the connection. It reports [errors.ErrConnectionClosed] when the connection has
// already been closed.
func (c *clientConn) Bind(uid int64) error {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}

	c.uid.Store(uid)

	c.rw.RUnlock()

	return nil
}

// Unbind removes the bound user ID. It reports [errors.ErrConnectionClosed] when the connection has
// already been closed.
func (c *clientConn) Unbind() error {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}

	c.uid.Store(0)

	c.rw.RUnlock()

	return nil
}

// Push sends a message.
//
// The message is written to the write queue and dispatched by the write goroutine. When the send
// fails, releasing the buffer is left to the caller.
func (c *clientConn) Push(buf buffer.Buffer) error {
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
func (c *clientConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close closes the connection.
//
// It closes immediately when force is true; otherwise it performs a graceful close.
func (c *clientConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose()
	} else {
		return c.graceClose()
	}
}

// LocalIP returns the local IP address.
func (c *clientConn) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr returns the local network address.
func (c *clientConn) LocalAddr() (net.Addr, error) {
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
func (c *clientConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr returns the remote network address.
func (c *clientConn) RemoteAddr() (net.Addr, error) {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return nil, errors.ErrConnectionClosed
	}

	conn := c.conn
	c.rw.RUnlock()

	return conn.RemoteAddr(), nil
}

// checkState checks the connection state.
//
// It returns the error matching the hanged or closed state, and nil when the connection is normal:
// [errors.ErrConnectionHanged] when hanged and [errors.ErrConnectionClosed] when closed.
func (c *clientConn) checkState() error {
	switch c.State() {
	case network.ConnHanged:
		return errors.ErrConnectionHanged
	case network.ConnClosed:
		return errors.ErrConnectionClosed
	default:
		return nil
	}
}

// graceClose closes the connection gracefully.
//
// It writes the close signal, waits for the write queue to drain and then closes the connection,
// so that as many buffered messages as possible are delivered. When a graceful close timeout is
// configured, a queue that has not drained in time disconnects the underlying connection directly
// to end the wait forcibly.
func (c *clientConn) graceClose() error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnHanged)) {
		return errors.ErrConnectionNotOpened
	}

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
		if closeTimeout := c.cli.opts.closeTimeout; closeTimeout > 0 {
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
func (c *clientConn) forceClose() error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose(true)
}

// doClose performs the close operation.
//
// It closes the write queue, waits for the read and write goroutines to exit, closes the KCP
// connection and finally triggers the disconnect hook. When force is true it closes the KCP
// connection first to interrupt any write operation that may be blocking in the write goroutine,
// which guarantees forced-close semantics.
func (c *clientConn) doClose(force ...bool) error {
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

	if c.cli.disconnectHandler != nil {
		c.cli.disconnectHandler(c)
	}

	return err
}

// read reads messages.
//
// It reads KCP data in a loop, checks the connection state and heartbeat packets, and hands valid
// messages to the receive hook.
func (c *clientConn) read(conn *kcp.UDPSession) {
	var index = 0

	for {
		isHeartbeat, heartbeatTime, buf, err := packet.Read(conn)
		if err != nil {
			taskpool.Add(func() { c.forceClose() })
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
			if c.cli.opts.heartbeatInterval > 0 {
				c.lastHeartbeatTime.Store(time.Now().UnixNano())
			}

			// trigger heartbeat handler
			if c.cli.heartbeatHandler != nil {
				c.cli.heartbeatHandler(c, heartbeatTime)
			}
		} else {
			// update heartbeat time
			if c.cli.opts.heartbeatInterval > 0 {
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

			if c.cli.receiveHandler != nil {
				c.cli.receiveHandler(c, buf)
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
func (c *clientConn) write(conn *kcp.UDPSession) {
	var tickerC <-chan time.Time

	if c.cli.opts.heartbeatInterval > 0 {
		ticker := time.NewTicker(c.cli.opts.heartbeatInterval)
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
func (c *clientConn) doBatchWrite(conn *kcp.UDPSession, first buffer.Buffer) {
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
		if c.cli.opts.writeTimeout > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(c.cli.opts.writeTimeout))
		}

		if _, err := conn.WriteBuffers(c.netBuffers); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Warnf("write message error: %v", err)
				taskpool.Add(func() { c.forceClose() })
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
// sends a heartbeat packet to the peer. It returns whether to keep running, which is false when the
// connection was forcibly closed for a heartbeat timeout.
func (c *clientConn) doHandleHeartbeat(conn *kcp.UDPSession, t time.Time) bool {
	if c.lastHeartbeatTime.Load() < t.Add(-2*c.cli.opts.heartbeatInterval).UnixNano() {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		taskpool.Add(func() { c.forceClose() })

		return false
	} else {
		if c.cli.opts.writeTimeout > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(c.cli.opts.writeTimeout))
		}

		hb := packet.PackHeartbeat()

		if _, err := conn.Write(hb.Bytes()); err != nil {
			log.Warnf("write heartbeat message error: %v", err)
		}

		hb.Release()

		return true
	}
}

// isClosed reports whether the connection state is closed.
func (c *clientConn) isClosed() bool {
	return c.State() == network.ConnClosed
}
