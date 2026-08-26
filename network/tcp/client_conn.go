package tcp

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
)

type clientConn struct {
	rw                sync.RWMutex        // 锁
	id                int64               // 连接ID
	uid               atomic.Int64        // 用户ID
	attr              *attr               // 连接属性
	conn              net.Conn            // TCP源连接
	state             atomic.Int32        // 连接状态
	client            *client             // 客户端
	wg1               *sync.WaitGroup     // 读等待组
	wg2               *sync.WaitGroup     // 写等待组
	lowPriorityQueue  *queue.Queue[*task] // 低优先级队列
	highPriorityQueue *queue.Queue[*task] // 高优先级队列
	lastHeartbeatTime atomic.Int64        // 上次心跳时间
}

var _ network.Conn = &clientConn{}

// newClientConn 创建一个客户端连接
// @param id int64 连接ID
// @param conn net.Conn TCP连接
// @param client *client 客户端
// @return @1 network.Conn 连接对象
func newClientConn(id int64, conn net.Conn, client *client) network.Conn {
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
	c.wg1.Go(c.read)
	c.wg2 = &sync.WaitGroup{}
	c.wg2.Go(c.write)

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
// @return @1 int64 用户ID
func (c *clientConn) UID() int64 {
	return c.uid.Load()
}

// Attr 获取属性接口
// @return @1 network.Attr 属性接口
func (c *clientConn) Attr() network.Attr {
	return c.attr
}

// Bind 绑定用户ID
// @param uid int64 用户ID
// @return @1 error 错误信息
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
// @return @1 error 错误信息
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
// @param msg []byte 消息内容
// @return @1 error 错误信息
func (c *clientConn) Send(msg []byte) (err error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	return c.doWriteToQueue(c.highPriorityQueue, dataPacket, msg)
}

// Push 低优先级发送消息
// @param msg []byte 消息内容
// @return @1 error 错误信息
func (c *clientConn) Push(msg []byte) error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	return c.doWriteToQueue(c.lowPriorityQueue, dataPacket, msg)
}

// State 获取连接状态
// @return @1 network.ConnState 连接状态
func (c *clientConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close 关闭连接（主动关闭）
// @param force ...bool 是否强制关闭
// @return @1 error 错误信息
func (c *clientConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose()
	} else {
		return c.graceClose()
	}
}

// LocalIP 获取本地IP
// @return @1 string 本地IP地址
// @return @2 error 错误信息
func (c *clientConn) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr 获取本地地址
// @return @1 net.Addr 本地地址
// @return @2 error 错误信息
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
// @return @2 error 错误信息
func (c *clientConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr 获取远端地址
// @return @1 net.Addr 远端地址
// @return @2 error 错误信息
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
// @return @1 error 错误信息
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
// @return @1 error 错误信息
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
// @return @1 error 错误信息
func (c *clientConn) forceClose() error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose()
}

// doClose 执行关闭操作
// @return @1 error 错误信息
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
func (c *clientConn) read() {
	conn := c.conn

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
// 为了保证心跳能够优先下发到客户端，故而实现一个优先队列
func (c *clientConn) write() {
	var (
		conn   = c.conn
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
// @param conn net.Conn TCP连接
// @param t *task 任务对象
func (c *clientConn) doWrite(conn net.Conn, t *task) {
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
		if !errors.Is(err, net.ErrClosed) {
			log.Errorf("write message error: %v", err)
		}
	}
}

// doHandleHeartbeat 处理心跳
// @param conn net.Conn TCP连接
// @param t time.Time 当前时间
// @return @1 bool 是否继续运行
func (c *clientConn) doHandleHeartbeat(conn net.Conn, t time.Time) bool {
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
// @return @1 bool 是否已关闭
func (c *clientConn) isClosed() bool {
	return c.State() == network.ConnClosed
}

// doWriteToQueue 写入任务到队列
// @param q *queue.Queue[*task] 队列
// @param typ int8 任务类型
// @param msg ...[]byte 消息内容
// @return @1 error 错误信息
func (c *clientConn) doWriteToQueue(q *queue.Queue[*task], typ int8, msg ...[]byte) error {
	t := c.client.allocateTask(typ, msg...)

	if err := q.Write(t); err != nil {
		c.client.recycleTask(t)
		return err
	}

	return nil
}
