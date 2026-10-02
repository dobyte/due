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

type clientConn struct {
	rw                sync.RWMutex                // Lock
	id                int64                       // Connection ID
	uid               atomic.Int64                // User ID
	attr              *attr                       // Connection attributes
	qc                *quic.Conn                  // QUIC connection
	stream            *quic.Stream                // Bidirectional stream
	state             atomic.Int32                // Connection state
	cli               *client                     // Client
	wg1               *sync.WaitGroup             // Read wait group
	wg2               *sync.WaitGroup             // Write wait group
	queue             *queue.Queue[buffer.Buffer] // Message queue
	output            *bufferWriter               // Write helper
	dueBuffers        []buffer.Buffer             // Buffers pending write
	lastHeartbeatTime atomic.Int64                // Time of the last heartbeat
}

var _ network.Conn = &clientConn{}

// newClientConn returns a new client connection.
func newClientConn(cli *client, qc *quic.Conn, stream *quic.Stream) network.Conn {
	c := &clientConn{}
	c.id = cli.cid.Add(1)
	c.attr = &attr{}
	c.qc = qc
	c.stream = stream
	c.cli = cli
	c.state.Store(int32(network.ConnOpened))
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(minWriteQueueSize, cli.opts.writeQueueSize)), cli.opts.writeTimeout)
	c.output = newBufferWriter(stream)
	c.dueBuffers = make([]buffer.Buffer, 0, maxBatchWriteNum)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.wg1 = &sync.WaitGroup{}
	c.wg2 = &sync.WaitGroup{}

	c.wg2.Go(func() { c.write(stream) })
	c.wg1.Go(func() {
		// The connect hook runs before the read loop so that it always fires before the
		// receive hook.
		if cli.connectHandler != nil {
			cli.connectHandler(c)
		}
		c.read(stream)
	})

	return c
}

// ID returns the connection ID.
func (c *clientConn) ID() int64 {
	return c.id
}

// UID returns the user ID, or 0 when none is bound.
func (c *clientConn) UID() int64 {
	return c.uid.Load()
}

// Attr returns the attribute interface.
func (c *clientConn) Attr() network.Attr {
	return c.attr
}

// Bind binds uid to the connection.
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

// Unbind removes the bound user ID.
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
// The message is written to the send queue. Ownership of buf is transferred to the network layer
// only when Push returns nil; otherwise releasing the buffer is left to the caller.
func (c *clientConn) Push(buf buffer.Buffer) error {
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
func (c *clientConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close closes the connection.
func (c *clientConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose()
	}
	return c.graceClose()
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

	qc := c.qc
	c.rw.RUnlock()

	return qc.LocalAddr(), nil
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

	qc := c.qc
	c.rw.RUnlock()

	return qc.RemoteAddr(), nil
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
	if c.qc == nil {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}
	qc := c.qc
	q := c.queue
	err := q.Write(buffer.NewBytes(nil))
	c.rw.RUnlock()

	if err == nil {
		if closeTimeout := c.cli.opts.closeTimeout; closeTimeout > 0 {
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
func (c *clientConn) forceClose() error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose(false)
}

// doClose performs the close operation.
//
// It closes the write queue, waits for the read and write goroutines to exit, closes the stream and
// the QUIC connection, and triggers the disconnect hook. When graceful is true it waits for the
// write goroutine to drain and sends FIN first, then dwells for closeTimeout to let the peer
// acknowledge before tearing the connection down.
func (c *clientConn) doClose(graceful bool) error {
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
		case <-time.After(c.cli.opts.closeTimeout):
		}
	}

	err := qc.CloseWithError(0, "closed")

	c.wg2.Wait()
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
// heartbeat packets and dispatches them to the receive hook; a read failure triggers a forced close.
func (c *clientConn) read(stream *quic.Stream) {
	var index = 0

	for {
		isHeartbeat, heartbeatTime, buf, err := packet.Read(stream)
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
			if c.cli.opts.heartbeatInterval > 0 {
				c.lastHeartbeatTime.Store(time.Now().UnixNano())
			}

			if c.cli.heartbeatHandler != nil {
				c.cli.heartbeatHandler(c, heartbeatTime)
			}
		} else {
			if c.cli.opts.heartbeatInterval > 0 {
				index++
				if index%10 == 0 {
					c.lastHeartbeatTime.Store(time.Now().UnixNano())
				}
			}

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
// It takes messages from the write queue in a batch and writes them to the stream, and triggers
// heartbeat detection and dispatch at the heartbeat interval.
func (c *clientConn) write(stream *quic.Stream) {
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
func (c *clientConn) doBatchWrite(stream *quic.Stream, first buffer.Buffer) {
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
		if c.cli.opts.writeTimeout > 0 {
			_ = stream.SetWriteDeadline(time.Now().Add(c.cli.opts.writeTimeout))
		}

		if err := c.output.write(buf); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Warnf("write message error: %v", err)
			taskpool.Add(func() { c.forceClose() })
		}

		buf.Release()
	}

	c.dueBuffers = c.dueBuffers[:0]
}

// doHandleHeartbeat handles heartbeats.
//
// It checks whether the time of the last received message has timed out and triggers a forced close
// when it has; otherwise it dispatches a heartbeat packet. It returns whether to keep the write
// goroutine looping, which is false on a heartbeat timeout.
func (c *clientConn) doHandleHeartbeat(stream *quic.Stream, t time.Time) bool {
	if c.lastHeartbeatTime.Load() < t.Add(-2*c.cli.opts.heartbeatInterval).UnixNano() {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		taskpool.Add(func() { c.forceClose() })

		return false
	}

	if c.cli.opts.writeTimeout > 0 {
		_ = stream.SetWriteDeadline(time.Now().Add(c.cli.opts.writeTimeout))
	}

	hb := packet.PackHeartbeat()

	if err := c.output.write(hb); err != nil {
		log.Warnf("write heartbeat message error: %v", err)
	}

	hb.Release()

	return true
}

// isClosed reports whether the connection state is closed.
func (c *clientConn) isClosed() bool {
	return c.State() == network.ConnClosed
}
