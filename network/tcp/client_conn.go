package tcp

import (
	"bufio"
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
)

type clientConn struct {
	rw                sync.RWMutex                // Lock
	id                int64                       // Connection ID
	uid               atomic.Int64                // User ID
	attr              *attr                       // Connection attributes
	conn              net.Conn                    // Underlying TCP connection
	state             atomic.Int32                // Connection state
	cli               *client                     // Client
	wg1               sync.WaitGroup              // Read wait group
	wg2               sync.WaitGroup              // Write wait group
	queue             *queue.Queue[buffer.Buffer] // Message queue
	dueBuffers        []buffer.Buffer             // Buffers waiting to be written
	netBuffers        net.Buffers                 // Byte slices waiting to be written
	lastHeartbeatTime atomic.Int64                // Time of the last received heartbeat
}

var _ network.Conn = &clientConn{}

// newClientConn returns a new client connection.
func newClientConn(cli *client, conn net.Conn) network.Conn {
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

	if c.cli.connectHandler != nil {
		c.cli.connectHandler(c)
	}

	return c
}

// ID returns the connection ID.
func (c *clientConn) ID() int64 {
	return c.id
}

// UID returns the user ID.
func (c *clientConn) UID() int64 {
	return c.uid.Load()
}

// Attr returns the attribute interface of the connection.
func (c *clientConn) Attr() network.Attr {
	return c.attr
}

// Bind binds the connection to the given user ID.
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

// Unbind unbinds the user ID from the connection.
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
// The caller controls when buf is released when the send fails.
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

// State returns the connection state.
func (c *clientConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close closes the connection.
//
// It is an active close, and none of the methods or attributes of the connection object may be used
// again afterwards. When force is true the connection is closed forcibly.
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

// LocalAddr returns the local address.
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

// RemoteAddr returns the remote address.
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

// checkState checks the connection state. It returns the matching error for the hanged or closed
// state, and nil when the connection is normal.
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
// It writes a close signal to the write queue and waits for the queue to drain before closing the
// connection, so that buffered messages can be delivered as far as possible. When a graceful close
// timeout is configured, the underlying connection is closed once the timeout elapses before the
// queue drains, which forces the wait to end.
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
func (c *clientConn) forceClose() error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose(true)
}

// doClose performs the close.
//
// It closes the write queue, waits for the read and write goroutines to exit, closes the TCP
// connection and finally invokes the disconnect handler. When force is true it closes the TCP
// connection first so that a possibly blocking write in the write goroutine is interrupted,
// preserving the force-close semantics.
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
// It keeps reading messages from the stream, updates the heartbeat time, detects empty and
// heartbeat packets and dispatches them to the receive handler. A read failure triggers a force
// close.
func (c *clientConn) read(conn net.Conn) {
	var (
		index  = 0
		reader = bufio.NewReaderSize(conn, c.cli.opts.readBufferSize)
	)

	for {
		isHeartbeat, heartbeatTime, buf, err := packet.Read(reader)
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
// It takes messages from the write queue in batches and writes them to the connection, and triggers
// heartbeat detection and delivery at the heartbeat interval.
func (c *clientConn) write(conn net.Conn) {
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
// It takes tasks from the write queue in batches, collects their bytes and writes them through
// net.Buffers at once to reduce the number of system calls.
func (c *clientConn) doBatchWrite(conn net.Conn, first buffer.Buffer) {
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

		if _, err := c.netBuffers.WriteTo(conn); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Warnf("write message error: %v", err)
				taskpool.Add(func() { c.forceClose() })
			}
		}
	}

	for _, buf := range c.dueBuffers {
		buf.Release()
	}

	c.dueBuffers = c.dueBuffers[:0]
	c.netBuffers = c.netBuffers[:0]
}

// doHandleHeartbeat handles heartbeats.
//
// It checks whether the time of the last received message has timed out and triggers a force close
// if so; otherwise it sends a heartbeat packet. It returns false when the write goroutine should
// stop because the heartbeat timed out.
func (c *clientConn) doHandleHeartbeat(conn net.Conn, t time.Time) bool {
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
