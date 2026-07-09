package quic

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xnet"
	"github.com/dobyte/due/v2/utils/xtime"
	"github.com/quic-go/quic-go"
)

type serverConn struct {
	id                int64               // 连接ID
	uid               atomic.Int64        // 用户ID
	attr              *attr               // 连接属性
	state             atomic.Int32        // 连接状态
	connMgr           *serverConnMgr      // 连接管理
	rw                sync.RWMutex        // 锁
	wg1               *sync.WaitGroup     // 读等待组
	wg2               *sync.WaitGroup     // 写等待组
	qc                quic.Connection     // QUIC连接
	stream            quic.Stream         // QUIC流
	lowPriorityQueue  *queue.Queue[*task] // 低优先级队列
	highPriorityQueue *queue.Queue[*task] // 高优先级队列
	lastHeartbeatTime atomic.Int64        // 上次心跳时间
	authorizeTimer    atomic.Value        // 授权定时器
}

var _ network.Conn = &serverConn{}

// ID 获取连接ID
func (c *serverConn) ID() int64 {
	return c.id
}

// UID 获取用户ID
func (c *serverConn) UID() int64 {
	return c.uid.Load()
}

// Attr 获取属性接口
func (c *serverConn) Attr() network.Attr {
	return c.attr
}

// Bind 绑定用户ID
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

// Unbind 解绑用户ID
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

// Send 高优先级发送消息
func (c *serverConn) Send(msg []byte) (err error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	return c.doWriteToQueue(c.highPriorityQueue, dataPacket, msg)
}

// Push 低优先级发送消息
func (c *serverConn) Push(msg []byte) error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	return c.doWriteToQueue(c.lowPriorityQueue, dataPacket, msg)
}

// State 获取连接状态
func (c *serverConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close 关闭连接
func (c *serverConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose(true)
	} else {
		return c.graceClose(true)
	}
}

// LocalIP 获取本地IP
func (c *serverConn) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr 获取本地地址
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

// RemoteIP 获取远端IP
func (c *serverConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr 获取远端地址
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

// 初始化连接
func (c *serverConn) init(qc quic.Connection, stream quic.Stream) {
	c.id = c.connMgr.id.Add(1)
	c.uid.Store(0)
	c.attr.values.Clear()
	c.state.Store(int32(network.ConnOpened))
	c.qc = qc
	c.stream = stream
	c.lowPriorityQueue = queue.NewQueue[*task](int32(max(128, c.connMgr.server.opts.writeQueueSize)), c.connMgr.server.opts.writeTimeout)
	c.highPriorityQueue = queue.NewQueue[*task](int32(max(128, c.connMgr.server.opts.writeQueueSize/2)), c.connMgr.server.opts.writeTimeout)
	c.lastHeartbeatTime.Store(xtime.Now().UnixNano())
	c.authorizeTimer.Store((*time.Timer)(nil))
	c.wg1 = &sync.WaitGroup{}
	c.wg1.Go(c.read)
	c.wg2 = &sync.WaitGroup{}
	c.wg2.Go(c.write)

	c.checkAuthorize()

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}
}

// 重置连接
func (c *serverConn) reset() {
	c.wg1 = nil
	c.wg2 = nil
	c.qc = nil
	c.stream = nil
	c.lowPriorityQueue = nil
	c.highPriorityQueue = nil
	c.attr.values.Clear()
	c.authorizeTimer.Store((*time.Timer)(nil))
}

// 检测连接状态
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

// 授权检查
func (c *serverConn) checkAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
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
}

// 取消授权检查
func (c *serverConn) uncheckAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		timer := c.authorizeTimer.Swap((*time.Timer)(nil))

		if t, ok := timer.(*time.Timer); ok && t != nil {
			t.Stop()
		}
	}
}

// 优雅关闭
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
	err1 := c.doWriteToQueue(c.lowPriorityQueue, closeSig)
	err2 := c.doWriteToQueue(c.highPriorityQueue, closeSig)
	c.rw.RUnlock()

	if err1 == nil {
		c.lowPriorityQueue.Wait()
	}

	if err2 == nil {
		c.highPriorityQueue.Wait()
	}

	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose(isNeedRecycle)
}

// 强制关闭
func (c *serverConn) forceClose(isNeedRecycle bool) error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	c.uncheckAuthorize()

	return c.doClose(isNeedRecycle)
}

// 执行关闭操作
func (c *serverConn) doClose(isNeedRecycle bool) error {
	c.rw.Lock()
	if c.qc == nil {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	c.lowPriorityQueue.Close()
	c.highPriorityQueue.Close()
	qc := c.qc
	stream := c.stream
	c.qc = nil
	c.stream = nil
	c.rw.Unlock()

	c.wg2.Wait()

	_ = stream.Close()
	err := qc.CloseWithError(0, "normal close")

	c.wg1.Wait()

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	if isNeedRecycle {
		c.connMgr.recycleConn(qc)
	}

	return err
}

// 读取消息
func (c *serverConn) read() {
	stream := c.stream

	for {
		data, err := packet.ReadMessage(stream)
		if err != nil {
			xcall.Go(func() {
				_ = c.forceClose(true)
			})
			return
		}

		if c.connMgr.server.opts.heartbeatInterval > 0 {
			c.lastHeartbeatTime.Store(xtime.Now().UnixNano())
		}

		// stop read message
		if c.checkState() != nil {
			return
		}

		// ignore empty packet
		if len(data) == 0 {
			continue
		}

		// check heartbeat packet
		isHeartbeat, err := packet.CheckHeartbeat(data)
		if err != nil {
			log.Errorf("check heartbeat message error: %v", err)
			continue
		}

		// ignore heartbeat packet
		if isHeartbeat {
			// responsive heartbeat
			if c.connMgr.server.opts.heartbeatMechanism == RespHeartbeat {
				c.rw.RLock()
				if c.stream != nil {
					if err := c.doWriteToQueue(c.highPriorityQueue, heartbeatPacket); err != nil {
						log.Errorf("write heartbeat packet to queue failed: %v", err)
					}
				}
				c.rw.RUnlock()
			}
		} else {
			if c.connMgr.server.receiveHandler != nil {
				c.connMgr.server.receiveHandler(c, data)
			}
		}
	}
}

// 写入消息
// 为了保证心跳能够优先下发到客户端，故而实现一个优先队列
func (c *serverConn) write() {
	var (
		stream = c.stream
		ticker *time.Ticker
	)

	if c.connMgr.server.opts.heartbeatInterval > 0 {
		ticker = time.NewTicker(c.connMgr.server.opts.heartbeatInterval)
		defer ticker.Stop()
	} else {
		ticker = &time.Ticker{}
	}

	for {
		select {
		case t, ok := <-c.highPriorityQueue.Read():
			if !ok {
				return
			}

			c.highPriorityQueue.Done(t.typ == closeSig)
			c.doWrite(stream, t)
		case t, ok := <-ticker.C:
			if !ok {
				return
			}

			if !c.doHandleHeartbeat(stream, t) {
				return
			}
		default:
			select {
			case t, ok := <-c.highPriorityQueue.Read():
				if !ok {
					return
				}

				c.highPriorityQueue.Done(t.typ == closeSig)
				c.doWrite(stream, t)
			case t, ok := <-c.lowPriorityQueue.Read():
				if !ok {
					return
				}

				c.lowPriorityQueue.Done(t.typ == closeSig)
				c.doWrite(stream, t)
			case t, ok := <-ticker.C:
				if !ok {
					return
				}

				if !c.doHandleHeartbeat(stream, t) {
					return
				}
			}
		}
	}
}

// 执行写入操作
func (c *serverConn) doWrite(stream quic.Stream, t *task) {
	defer c.connMgr.recycleTask(t)

	if t.typ == closeSig {
		return
	}

	if t.typ == heartbeatPacket {
		if msg, err := packet.PackHeartbeat(); err != nil {
			log.Errorf("pack heartbeat message error: %v", err)
			return
		} else {
			t.msg = msg
		}
	}

	if _, err := stream.Write(t.msg); err != nil {
		log.Errorf("write message error: %v", err)
	}
}

// 处理心跳
func (c *serverConn) doHandleHeartbeat(stream quic.Stream, t time.Time) bool {
	deadline := t.Add(-2 * c.connMgr.server.opts.heartbeatInterval).UnixNano()

	if c.lastHeartbeatTime.Load() < deadline {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		xcall.Go(func() {
			_ = c.forceClose(true)
		})

		return false
	} else {
		if c.connMgr.server.opts.heartbeatMechanism == TickHeartbeat {
			if heartbeat, err := packet.PackHeartbeat(); err != nil {
				log.Errorf("pack heartbeat message error: %v", err)
			} else {
				// send heartbeat packet
				if _, err := stream.Write(heartbeat); err != nil {
					log.Errorf("write heartbeat message error: %v", err)
				}
			}
		}
	}

	return true
}

// 是否已关闭
func (c *serverConn) isClosed() bool {
	return c.State() == network.ConnClosed
}

// 写入任务到队列
func (c *serverConn) doWriteToQueue(q *queue.Queue[*task], typ int8, msg ...[]byte) error {
	t := c.connMgr.allocateTask(typ, msg...)

	if err := q.Write(t); err != nil {
		c.connMgr.recycleTask(t)
		return err
	}

	return nil
}
