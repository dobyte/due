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
	qc                *quic.Conn          // QUIC连接
	stream            *quic.Stream        // QUIC流
	lowPriorityQueue  *queue.Queue[*task] // 低优先级队列
	highPriorityQueue *queue.Queue[*task] // 高优先级队列
	lastHeartbeatTime atomic.Int64        // 上次心跳时间
	authorizeTimer    atomic.Value        // 授权定时器
}

var _ network.Conn = &serverConn{}

// ID 获取连接ID
// @return @1 int64 连接ID
func (c *serverConn) ID() int64 {
	return c.id
}

// UID 获取用户ID
// @return @1 int64 已绑定的用户ID，未绑定时为0
func (c *serverConn) UID() int64 {
	return c.uid.Load()
}

// Attr 获取属性接口
// @return @1 network.Attr 连接属性接口，用于读写自定义属性
func (c *serverConn) Attr() network.Attr {
	return c.attr
}

// Bind 绑定用户ID
// 绑定成功后取消授权超时检测
// @param uid int64 待绑定的用户ID
// @return @1 error 连接已关闭时返回errors.ErrConnectionClosed
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
// 解绑后重新开启授权超时检测
// @return @1 error 连接已关闭时返回errors.ErrConnectionClosed
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
// 消息写入高优先级队列，保证心跳等关键消息优先下发
// @param msg []byte 待发送的消息字节
// @return @1 error 连接状态异常或队列写入失败时返回的错误
func (c *serverConn) Send(msg []byte) (err error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	return c.doWriteToQueue(c.highPriorityQueue, dataPacket, msg)
}

// Push 低优先级发送消息
// 消息写入低优先级队列，在高优先级队列空闲时才会被下发
// @param msg []byte 待发送的消息字节
// @return @1 error 连接状态异常或队列写入失败时返回的错误
func (c *serverConn) Push(msg []byte) error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	return c.doWriteToQueue(c.lowPriorityQueue, dataPacket, msg)
}

// State 获取连接状态
// @return @1 network.ConnState 当前连接状态
func (c *serverConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close 关闭连接
// @param force ...bool 是否强制关闭；为true时立即关闭，缺省或为false时执行优雅关闭
// @return @1 error 关闭失败或连接已处于关闭态时返回的错误
func (c *serverConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose(true)
	} else {
		return c.graceClose(true)
	}
}

// LocalIP 获取本地IP
// @return @1 string 本地IP地址
// @return @2 error 连接已关闭或地址解析失败时返回的错误
func (c *serverConn) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr 获取本地地址
// @return @1 net.Addr 本地网络地址
// @return @2 error 连接已关闭时返回的错误
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
// @return @1 string 远端IP地址
// @return @2 error 连接已关闭或地址解析失败时返回的错误
func (c *serverConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr 获取远端地址
// @return @1 net.Addr 远端网络地址
// @return @2 error 连接已关闭时返回的错误
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

// init 初始化连接
// 复用对象池中的连接对象，重置各项状态、创建读写协程并执行授权检查与连接钩子
// @param qc *quic.Conn 与之关联的QUIC连接
// @param stream *quic.Stream 与之关联的QUIC流
func (c *serverConn) init(qc *quic.Conn, stream *quic.Stream) {
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
	c.wg1.Go(func() { c.read(stream) })
	c.wg2 = &sync.WaitGroup{}
	c.wg2.Go(func() { c.write(stream) })

	c.checkAuthorize()

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}
}

// reset 重置连接
// 清空连接对象内的引用与状态，以便归还对象池后安全复用
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

// checkState 检测连接状态
// 依据挂起/关闭状态返回对应错误，正常时返回nil
// @return @1 error 挂起返回ErrConnectionHanged，关闭返回ErrConnectionClosed，正常为nil
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

// checkAuthorize 授权检查
// 开启授权超时定时器，超时且仍未绑定用户ID时强制关闭连接；重新创建前会停止旧定时器
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

// uncheckAuthorize 取消授权检查
// 停止授权超时定时器，用于绑定用户ID或关闭连接时解除授权检测
func (c *serverConn) uncheckAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		timer := c.authorizeTimer.Swap((*time.Timer)(nil))

		if t, ok := timer.(*time.Timer); ok && t != nil {
			t.Stop()
		}
	}
}

// graceClose 优雅关闭
// 写入关闭信号等待写队列排空后关闭连接，便于尽量下发完已缓冲的消息
// @param isNeedRecycle bool 是否在关闭后将连接对象归还连接池
// @return @1 error 连接非打开态或关闭过程中出错时返回的错误
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

// forceClose 强制关闭
// 立即切换状态为关闭并关闭连接，不等待写队列排空
// @param isNeedRecycle bool 是否在关闭后将连接对象归还连接池
// @return @1 error 连接已处于关闭态时返回的错误
func (c *serverConn) forceClose(isNeedRecycle bool) error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	c.uncheckAuthorize()

	return c.doClose(isNeedRecycle)
}

// doClose 执行关闭操作
// 关闭写队列，等待读写协程退出后关闭流与QUIC连接，触发断开hook，并按需归还连接对象
// @param isNeedRecycle bool 是否在关闭后将连接对象归还连接池
// @return @1 error 关闭QUIC连接时的错误
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

// read 读取消息
// 持续从流中读取消息，处理心跳（支持响应式心跳回复）与状态检测，并分发到接收hook；读取失败时触发强制关闭
// @param stream *quic.Stream 当前连接的QUIC流
func (c *serverConn) read(stream *quic.Stream) {
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
				err := c.doWriteToQueue(c.highPriorityQueue, heartbeatPacket)
				c.rw.RUnlock()
				if err != nil {
					log.Errorf("write heartbeat packet to queue failed: %v", err)
				}
			}
		} else {
			if c.connMgr.server.receiveHandler != nil {
				c.connMgr.server.receiveHandler(c, data)
			}
		}
	}
}

// write 写入消息
// 为保证心跳能够优先下发到客户端，采用高/低优先级双队列：外层先取高优先级，空闲时在内层再取低优先级或处理心跳
// @param stream *quic.Stream 当前连接的QUIC流
func (c *serverConn) write(stream *quic.Stream) {
	var ticker *time.Ticker
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

// doWrite 执行写入操作
// 依据任务类型打包心跳或直接写入消息字节，并回收任务对象
// @param stream *quic.Stream 当前连接的QUIC流
// @param t *task 待写入的任务对象
func (c *serverConn) doWrite(stream *quic.Stream, t *task) {
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

// doHandleHeartbeat 处理心跳
// 检测上次收到消息的时间是否超时，超时则触发强制关闭；主动定时心跳模式下额外下发心跳包
// @param stream *quic.Stream 当前连接的QUIC流
// @param t time.Time 当前心跳触发的时间点
// @return @1 bool 是否继续写入协程循环，心跳超时时返回false
func (c *serverConn) doHandleHeartbeat(stream *quic.Stream, t time.Time) bool {
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

// isClosed 是否已关闭
// @return @1 bool 连接状态是否为关闭
func (c *serverConn) isClosed() bool {
	return c.State() == network.ConnClosed
}

// doWriteToQueue 写入任务到队列
// 从对象池分配任务写入指定队列，写入失败时回收任务并返回错误
// @param q *queue.Queue[*task] 目标写队列
// @param typ int8 任务类型
// @param msg ...[]byte 待发送的消息字节，可缺省
// @return @1 error 队列挂起/关闭或写入超时时返回的错误
func (c *serverConn) doWriteToQueue(q *queue.Queue[*task], typ int8, msg ...[]byte) error {
	t := c.connMgr.allocateTask(typ, msg...)

	if err := q.Write(t); err != nil {
		c.connMgr.recycleTask(t)
		return err
	}

	return nil
}
