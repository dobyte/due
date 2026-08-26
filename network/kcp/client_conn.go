package kcp

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
	"github.com/xtaci/kcp-go/v5"
)

type clientConn struct {
	rw                sync.RWMutex        // 锁
	id                int64               // 连接ID
	uid               atomic.Int64        // 用户ID
	attr              *attr               // 连接属性
	conn              *kcp.UDPSession     // UDP源连接
	state             atomic.Int32        // 连接状态
	client            *client             // 客户端
	wg1               *sync.WaitGroup     // 读等待组
	wg2               *sync.WaitGroup     // 写等待组
	lowPriorityQueue  *queue.Queue[*task] // 低优先级队列
	highPriorityQueue *queue.Queue[*task] // 高优先级队列
	lastHeartbeatTime atomic.Int64        // 上次心跳时间
}

var _ network.Conn = &clientConn{}

// newClientConn 创建客户端连接
// 初始化连接状态、读写队列及两路读写协程，并应用客户端相关KCP参数
// @param id int64 连接ID
// @param conn *kcp.UDPSession KCP源连接
// @param client *client 客户端实例
// @return @1 network.Conn 客户端连接实例
func newClientConn(id int64, conn *kcp.UDPSession, client *client) network.Conn {
	c := &clientConn{}
	c.id = id
	c.attr = &attr{}
	c.conn = conn
	c.client = client
	c.state.Store(int32(network.ConnOpened))
	c.lowPriorityQueue = queue.NewQueue[*task](int32(max(128, client.opts.writeQueueSize)), client.opts.writeTimeout)
	c.highPriorityQueue = queue.NewQueue[*task](int32(max(128, client.opts.writeQueueSize/2)), client.opts.writeTimeout)
	c.lastHeartbeatTime.Store(xtime.Now().UnixNano())
	c.wg1 = &sync.WaitGroup{}
	c.wg1.Go(func() { c.read(conn) })
	c.wg2 = &sync.WaitGroup{}
	c.wg2.Go(func() { c.write(conn) })

	if c.client.opts.mtu > 0 {
		conn.SetMtu(c.client.opts.mtu)
	}

	if len(c.client.opts.noDelay) == 4 {
		conn.SetNoDelay(c.client.opts.noDelay[0], c.client.opts.noDelay[1], c.client.opts.noDelay[2], c.client.opts.noDelay[3])
	}

	if c.client.opts.ackNoDelay {
		conn.SetACKNoDelay(c.client.opts.ackNoDelay)
	}

	if c.client.opts.writeDelay {
		conn.SetWriteDelay(c.client.opts.writeDelay)
	}

	if len(c.client.opts.windowSize) == 2 {
		conn.SetWindowSize(c.client.opts.windowSize[0], c.client.opts.windowSize[1])
	}

	if c.client.opts.readBuffer > 0 {
		conn.SetReadBuffer(c.client.opts.readBuffer)
	}

	if c.client.opts.writeBuffer > 0 {
		conn.SetWriteBuffer(c.client.opts.writeBuffer)
	}

	if c.client.connectHandler != nil {
		c.client.connectHandler(c)
	}

	return c
}

// ID 获取连接ID
// @return @1 int64 连接ID
func (c *clientConn) ID() int64 {
	return c.id
}

// UID 获取用户ID
// @return @1 int64 已绑定的用户ID，未绑定时为0
func (c *clientConn) UID() int64 {
	return c.uid.Load()
}

// Attr 获取属性接口
// @return @1 network.Attr 连接属性接口，用于读写自定义属性
func (c *clientConn) Attr() network.Attr {
	return c.attr
}

// Bind 绑定用户ID
// @param uid int64 待绑定的用户ID
// @return @1 error 连接已关闭时返回errors.ErrConnectionClosed
func (c *clientConn) Bind(uid int64) error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if c.isClosed() {
		return errors.ErrConnectionClosed
	}

	c.uid.Store(uid)

	return nil
}

// Unbind 解绑用户ID
// @return @1 error 连接已关闭时返回errors.ErrConnectionClosed
func (c *clientConn) Unbind() error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if c.isClosed() {
		return errors.ErrConnectionClosed
	}

	c.uid.Store(0)

	return nil
}

// Send 高优先级发送消息
// 消息写入高优先级队列，保证心跳等关键消息优先下发
// @param msg []byte 待发送的消息字节
// @return @1 error 连接状态异常或队列写入失败时返回的错误
func (c *clientConn) Send(msg []byte) (err error) {
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
func (c *clientConn) Push(msg []byte) error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	return c.doWriteToQueue(c.lowPriorityQueue, dataPacket, msg)
}

// State 获取连接状态
// @return @1 network.ConnState 当前连接状态
func (c *clientConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close 关闭连接（主动关闭）
// @param force ...bool 是否强制关闭；为true时立即关闭，缺省或为false时执行优雅关闭
// @return @1 error 关闭失败或连接已处于关闭态时返回的错误
func (c *clientConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose()
	} else {
		return c.graceClose()
	}
}

// LocalIP 获取本地IP
// @return @1 string 本地IP地址
// @return @2 error 连接已关闭或地址解析失败时返回的错误
func (c *clientConn) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr 获取本地地址
// @return @1 net.Addr 本地网络地址
// @return @2 error 连接已关闭时返回的错误
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

// RemoteIP 获取远端IP
// @return @1 string 远端IP地址
// @return @2 error 连接已关闭或地址解析失败时返回的错误
func (c *clientConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr 获取远端地址
// @return @1 net.Addr 远端网络地址
// @return @2 error 连接已关闭时返回的错误
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

// checkState 检测连接状态
// 依据挂起/关闭状态返回对应错误，正常时返回nil
// @return @1 error 挂起返回ErrConnectionHanged，关闭返回ErrConnectionClosed，正常为nil
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

// graceClose 优雅关闭
// 向两个写队列写入关闭信号，等待队列排空后关闭连接，便于尽量下发完已缓冲的消息
// @return @1 error 连接非打开态或关闭过程中出错时返回的错误
func (c *clientConn) graceClose() error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnHanged)) {
		return errors.ErrConnectionNotOpened
	}

	c.rw.RLock()
	if c.conn == nil {
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

	return c.doClose()
}

// forceClose 强制关闭
// 直接切换连接状态为关闭并执行关闭操作，不等待队列排空
// @return @1 error 连接已处于关闭态或关闭过程中出错时返回的错误
func (c *clientConn) forceClose() error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose()
}

// doClose 执行关闭操作
// 关闭读写队列，等待写协程退出后关闭底层连接，并触发断开hook函数
// @return @1 error 连接已关闭或关闭底层连接失败时返回的错误
func (c *clientConn) doClose() error {
	c.rw.Lock()
	if c.conn == nil {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	c.lowPriorityQueue.Close()
	c.highPriorityQueue.Close()
	conn := c.conn
	c.conn = nil
	c.rw.Unlock()

	c.wg2.Wait()

	err := conn.Close()

	c.wg1.Wait()

	if c.client.disconnectHandler != nil {
		c.client.disconnectHandler(c)
	}

	return err
}

// read 读取消息
// 循环读取KCP数据，校验连接状态与心跳包，并将有效消息交给接收hook函数处理
// @param conn *kcp.UDPSession KCP连接
func (c *clientConn) read(conn *kcp.UDPSession) {
	for {
		data, err := packet.ReadMessage(conn)
		if err != nil {
			xcall.Go(func() {
				_ = c.forceClose()
			})
			return
		}

		if c.client.opts.heartbeatInterval > 0 {
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
			continue
		}

		if c.client.receiveHandler != nil {
			c.client.receiveHandler(c, data)
		}
	}
}

// write 写入消息
// 从高低优先级队列及心跳定时器中选择待写入数据，为了保证心跳能够优先下发到客户端，故而实现一个优先队列
// @param conn *kcp.UDPSession KCP连接
func (c *clientConn) write(conn *kcp.UDPSession) {
	var (
		ticker *time.Ticker
	)

	if c.client.opts.heartbeatInterval > 0 {
		ticker = time.NewTicker(c.client.opts.heartbeatInterval)
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
			c.doWrite(conn, t)
		case t, ok := <-ticker.C:
			if !ok {
				return
			}

			if !c.doHandleHeartbeat(conn, t) {
				return
			}
		default:
			select {
			case t, ok := <-c.highPriorityQueue.Read():
				if !ok {
					return
				}

				c.highPriorityQueue.Done(t.typ == closeSig)
				c.doWrite(conn, t)
			case t, ok := <-c.lowPriorityQueue.Read():
				if !ok {
					return
				}

				c.lowPriorityQueue.Done(t.typ == closeSig)
				c.doWrite(conn, t)
			case t, ok := <-ticker.C:
				if !ok {
					return
				}

				if !c.doHandleHeartbeat(conn, t) {
					return
				}
			}
		}
	}
}

// doWrite 执行写入操作
// 根据任务类型组装心跳包并写入底层连接，完成后回收任务对象
// @param conn *kcp.UDPSession KCP连接
// @param t *task 待写入的任务对象
func (c *clientConn) doWrite(conn *kcp.UDPSession, t *task) {
	defer c.client.recycleTask(t)

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

	if _, err := conn.Write(t.msg); err != nil {
		log.Errorf("write message error: %v", err)
	}
}

// doHandleHeartbeat 处理心跳
// 超过心跳超时阈值则强制关闭连接，否则向对端发送心跳包
// @param conn *kcp.UDPSession KCP连接
// @param t time.Time 当前心跳时刻
// @return @1 bool 是否继续运行（心跳超时强制关闭返回false）
func (c *clientConn) doHandleHeartbeat(conn *kcp.UDPSession, t time.Time) bool {
	deadline := t.Add(-2 * c.client.opts.heartbeatInterval).UnixNano()

	if c.lastHeartbeatTime.Load() < deadline {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		xcall.Go(func() {
			_ = c.forceClose()
		})

		return false
	} else {
		if heartbeat, err := packet.PackHeartbeat(); err != nil {
			log.Errorf("pack heartbeat message error: %v", err)
		} else {
			// send heartbeat packet
			if _, err := conn.Write(heartbeat); err != nil {
				log.Errorf("write heartbeat message error: %v", err)
			}
		}
	}

	return true
}

// isClosed 是否已关闭
// @return @1 bool 连接状态是否为关闭
func (c *clientConn) isClosed() bool {
	return c.State() == network.ConnClosed
}

// doWriteToQueue 写入任务到队列
// 从对象池分配任务并写入指定队列，写入失败时回收任务对象
// @param q *queue.Queue[*task] 目标写入队列
// @param typ int8 任务类型
// @param msg ...[]byte 待发送的消息字节，可选
// @return @1 error 队列写入失败或已关闭时返回的错误
func (c *clientConn) doWriteToQueue(q *queue.Queue[*task], typ int8, msg ...[]byte) error {
	t := c.client.allocateTask(typ, msg...)

	if err := q.Write(t); err != nil {
		c.client.recycleTask(t)
		return err
	}

	return nil
}
