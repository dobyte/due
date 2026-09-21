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
	id                int64                       // 连接ID
	uid               atomic.Int64                // 用户ID
	attr              *attr                       // 连接属性
	state             atomic.Int32                // 连接状态
	connMgr           *serverConnMgr              // 连接管理器
	rw                sync.RWMutex                // 锁
	wg1               *sync.WaitGroup             // 读等待组
	wg2               *sync.WaitGroup             // 写等待组
	qc                *quic.Conn                  // QUIC连接
	stream            *quic.Stream                // 双向流
	queue             *queue.Queue[buffer.Buffer] // 消息队列
	output            *bufferWriter               // 写辅助对象
	lastHeartbeatTime atomic.Int64                // 上次心跳时间
	authorizeTimer    atomic.Value                // 授权定时器
}

var _ network.Conn = &serverConn{}

// ID returns the connection ID.
func (c *serverConn) ID() int64 {
	return c.id
}

// UID returns the bound user ID, or zero if unbound.
func (c *serverConn) UID() int64 {
	return c.uid.Load()
}

// Attr returns the connection attributes.
func (c *serverConn) Attr() network.Attr {
	return c.attr
}

// Bind binds a user ID and cancels the authorization timer.
func (c *serverConn) Bind(uid int64) error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if c.isClosed() {
		return errors.ErrConnectionClosed
	}

	c.uid.Store(uid)
	c.uncheckAuthorize()

	return nil
}

// Unbind clears the bound user ID and restarts the authorization timer.
func (c *serverConn) Unbind() error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if c.isClosed() {
		return errors.ErrConnectionClosed
	}

	c.uid.Store(0)
	c.checkAuthorize()

	return nil
}

// Push enqueues a message for delivery. Ownership of buf transfers to the
// network layer only when it returns nil.
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

// State returns the current connection state.
func (c *serverConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close closes the connection.
func (c *serverConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose(true)
	}
	return c.graceClose(true)
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

// init reuses a pooled connection object for a new session.
func (c *serverConn) init(id int64, qc *quic.Conn, stream *quic.Stream) {
	c.id = id
	c.uid.Store(0)
	c.attr.values.Clear()
	c.state.Store(int32(network.ConnOpened))
	c.qc = qc
	c.stream = stream
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(128, c.connMgr.server.opts.writeQueueSize)), c.connMgr.server.opts.writeTimeout)
	c.output = newBufferWriter(stream)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.authorizeTimer.Store((*time.Timer)(nil))
	c.wg1 = &sync.WaitGroup{}
	c.wg2 = &sync.WaitGroup{}

	c.wg2.Go(func() { c.write(stream) })
	c.wg1.Go(func() {
		c.checkAuthorize()

		if c.connMgr.server.connectHandler != nil {
			c.connMgr.server.connectHandler(c)
		}

		c.read(stream)
	})
}

// reset clears the connection fields so the object can be reused safely.
func (c *serverConn) reset() {
	c.wg1 = nil
	c.wg2 = nil
	c.qc = nil
	c.stream = nil
	c.queue = nil
	c.output = nil
	c.attr.values.Clear()
	c.authorizeTimer.Store((*time.Timer)(nil))
}

// checkState returns an error matching the current connection state.
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

// checkAuthorize arms the authorization timer that force-closes the connection
// when it remains unbound after the timeout.
func (c *serverConn) checkAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout <= 0 {
		return
	}

	cid := c.ID()

	timer := c.authorizeTimer.Swap(time.AfterFunc(c.connMgr.server.opts.authorizeTimeout, func() {
		if c.UID() != 0 {
			return
		}

		if c.ID() != cid {
			return
		}

		c.forceClose(true)
	}))

	if t, ok := timer.(*time.Timer); ok && t != nil {
		t.Stop()
	}
}

// uncheckAuthorize stops and clears the authorization timer.
func (c *serverConn) uncheckAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout <= 0 {
		return
	}

	timer := c.authorizeTimer.Swap((*time.Timer)(nil))

	if t, ok := timer.(*time.Timer); ok && t != nil {
		t.Stop()
	}
}

// graceClose drains the write queue before closing the underlying transport.
func (c *serverConn) graceClose(isNeedRecycle bool) error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnHanged)) {
		return errors.ErrConnectionNotOpened
	}

	c.uncheckAuthorize()

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

	return c.doClose(true, isNeedRecycle)
}

// forceClose closes the connection immediately without draining.
func (c *serverConn) forceClose(isNeedRecycle bool) error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	c.uncheckAuthorize()

	return c.doClose(false, isNeedRecycle)
}

// doClose closes the write queue, waits for the read and write workers to exit,
// closes the stream and QUIC transport, fires the disconnect handler and, when
// requested, recycles the connection object.
func (c *serverConn) doClose(graceful, isNeedRecycle bool) error {
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

	if isNeedRecycle {
		c.connMgr.recycleConn(c)
	}

	return err
}

// read reads messages from the stream and dispatches them to handlers.
func (c *serverConn) read(stream *quic.Stream) {
	var index = 0

	for {
		isHeartbeat, heartbeatTime, buf, err := packet.Read(stream)
		if err != nil {
			taskpool.Add(func() { c.forceClose(true) })
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

// write drains the write queue and sends periodic heartbeats.
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
func (c *serverConn) doWrite(stream *quic.Stream, buf buffer.Buffer) {
	closeSig := buf.Len() == 0

	c.queue.Done(closeSig)

	if closeSig {
		buf.Release()
		return
	}

	if c.connMgr.server.opts.writeTimeout > 0 {
		_ = stream.SetWriteDeadline(time.Now().Add(c.connMgr.server.opts.writeTimeout))
	}

	if err := c.output.write(buf); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Errorf("write message error: %v", err)
		taskpool.Add(func() { c.forceClose(true) })
	}

	buf.Release()
}

// doHandleHeartbeat checks the heartbeat timeout and, for the tick mechanism,
// sends a heartbeat.
func (c *serverConn) doHandleHeartbeat(stream *quic.Stream, t time.Time) bool {
	if c.lastHeartbeatTime.Load() < t.Add(-2*c.connMgr.server.opts.heartbeatInterval).UnixNano() {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		taskpool.Add(func() { c.forceClose(true) })

		return false
	}

	if c.connMgr.server.opts.heartbeatMechanism == TickHeartbeat {
		if c.connMgr.server.opts.writeTimeout > 0 {
			_ = stream.SetWriteDeadline(time.Now().Add(c.connMgr.server.opts.writeTimeout))
		}

		hb := packet.PackHeartbeat(true)

		if err := c.output.write(hb); err != nil {
			log.Errorf("write heartbeat message error: %v", err)
		}

		hb.Release()
	}

	return true
}

// isClosed reports whether the connection is closed.
func (c *serverConn) isClosed() bool {
	return c.State() == network.ConnClosed
}
