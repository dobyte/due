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
	rw                sync.RWMutex                // 锁
	id                int64                       // 连接ID
	uid               atomic.Int64                // 用户ID
	attr              *attr                       // 连接属性
	qc                *quic.Conn                  // QUIC连接
	stream            *quic.Stream                // 双向流
	state             atomic.Int32                // 连接状态
	cli               *client                     // 客户端
	wg1               *sync.WaitGroup             // 读等待组
	wg2               *sync.WaitGroup             // 写等待组
	queue             *queue.Queue[buffer.Buffer] // 消息队列
	output            *bufferWriter               // 写辅助对象
	lastHeartbeatTime atomic.Int64                // 上次心跳时间
}

var _ network.Conn = &clientConn{}

// newClientConn creates a client connection.
func newClientConn(cli *client, qc *quic.Conn, stream *quic.Stream) network.Conn {
	c := &clientConn{}
	c.id = cli.cid.Add(1)
	c.attr = &attr{}
	c.qc = qc
	c.stream = stream
	c.cli = cli
	c.state.Store(int32(network.ConnOpened))
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(128, cli.opts.writeQueueSize)), cli.opts.writeTimeout)
	c.output = newBufferWriter(stream)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.wg1 = &sync.WaitGroup{}
	c.wg2 = &sync.WaitGroup{}

	c.wg2.Go(func() { c.write(stream) })
	c.wg1.Go(func() {
		// OnConnect runs before the read loop so it always precedes OnReceive.
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

// UID returns the bound user ID, or zero if unbound.
func (c *clientConn) UID() int64 {
	return c.uid.Load()
}

// Attr returns the connection attributes.
func (c *clientConn) Attr() network.Attr {
	return c.attr
}

// Bind binds a user ID.
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

// Unbind clears the bound user ID.
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

// Push enqueues a message for delivery. Ownership of buf transfers to the
// network layer only when it returns nil.
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

// State returns the current connection state.
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

// checkState returns an error matching the current connection state.
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

// graceClose drains the write queue before closing the underlying transport.
func (c *clientConn) graceClose() error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnHanged)) {
		return errors.ErrConnectionNotOpened
	}

	c.rw.RLock()
	if c.qc == nil {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}
	err := c.queue.Write(buffer.NewBytes(nil))
	c.rw.RUnlock()

	if err == nil {
		c.queue.Wait()
	}

	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose(true)
}

// forceClose closes the connection immediately without draining.
func (c *clientConn) forceClose() error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose(false)
}

// doClose closes the write queue, waits for the read and write workers to exit,
// closes the stream and QUIC transport, then fires the disconnect handler.
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
		// Wait for the writer to flush every accepted message before sending FIN.
		c.wg2.Wait()

		_ = stream.Close()

		// Keep the transport alive for closeTimeout so the peer can acknowledge
		// the FIN and retransmit, then tear it down.
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

// read reads messages from the stream and dispatches them to handlers.
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

// write drains the write queue and sends periodic heartbeats.
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

			c.doWrite(stream, buf)
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

// doWrite writes a single message to the stream.
func (c *clientConn) doWrite(stream *quic.Stream, buf buffer.Buffer) {
	closeSig := buf.Len() == 0

	c.queue.Done(closeSig)

	if closeSig {
		buf.Release()
		return
	}

	if c.cli.opts.writeTimeout > 0 {
		_ = stream.SetWriteDeadline(time.Now().Add(c.cli.opts.writeTimeout))
	}

	if err := c.output.write(buf); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Errorf("write message error: %v", err)
		taskpool.Add(func() { c.forceClose() })
	}

	buf.Release()
}

// doHandleHeartbeat checks the heartbeat timeout and sends a heartbeat.
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
		log.Errorf("write heartbeat message error: %v", err)
	}

	hb.Release()

	return true
}

// isClosed reports whether the connection is closed.
func (c *clientConn) isClosed() bool {
	return c.State() == network.ConnClosed
}
