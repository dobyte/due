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

type clientConn struct {
	rw                sync.RWMutex                // 锁
	id                int64                       // 连接ID
	uid               atomic.Int64                // 用户ID
	attr              *attr                       // 连接属性
	conn              *websocket.Conn             // TCP源连接
	state             atomic.Int32                // 连接状态
	cli               *client                     // 客户端
	wg1               *sync.WaitGroup             // 读等待组
	wg2               *sync.WaitGroup             // 写等待组
	queue             *queue.Queue[buffer.Buffer] // 消息队列
	lastHeartbeatTime atomic.Int64                // 上次心跳时间
}

var _ network.Conn = &clientConn{}

// newClientConn 创建一个客户端连接
// @param cli *client 客户端
// @param conn *websocket.Conn WS连接
// @return @1 network.Conn 连接对象
func newClientConn(cli *client, conn *websocket.Conn) network.Conn {
	c := &clientConn{}
	c.id = cli.cid.Add(1)
	c.cli = cli
	c.attr = &attr{}
	c.conn = conn
	c.state.Store(int32(network.ConnOpened))
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(128, cli.opts.writeQueueSize)), cli.opts.writeTimeout)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.wg1 = &sync.WaitGroup{}
	c.wg1.Go(func() { c.read(conn) })
	c.wg2 = &sync.WaitGroup{}
	c.wg2.Go(func() { c.write(conn) })

	if c.cli.connectHandler != nil {
		c.cli.connectHandler(c)
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

	if c.isClosed() {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}

	c.uid.Store(uid)

	c.rw.RUnlock()

	return nil
}

// Unbind 解绑用户ID
// @return @1 error 错误信息
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

// Push 低优先级发送消息
// @param buf buffer.Buffer 消息内容，消息发送失败自行控制释放buffer
// @return @1 error 错误信息
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
	err := c.queue.Write(buffer.NewBytes(nil))
	c.rw.RUnlock()

	if err == nil {
		c.queue.Wait()
	}

	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose()
}

// forceClose 强制关闭
// 立即切换状态为关闭并关闭连接，不等待写队列排空
// @return @1 error 连接已处于关闭态时返回的错误
func (c *clientConn) forceClose() error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose()
}

// doClose 执行关闭操作
// 关闭写队列，等待读写协程退出后关闭WS连接，最后触发断开hook
// @return @1 error 关闭WS连接时的错误
func (c *clientConn) doClose() error {
	c.rw.Lock()
	if c.conn == nil {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	c.queue.Close()
	conn := c.conn
	c.conn = nil
	c.rw.Unlock()

	c.wg2.Wait()

	err := conn.Close()

	c.wg1.Wait()

	for buf := range c.queue.Read() {
		buf.Release()
	}

	if c.cli.disconnectHandler != nil {
		c.cli.disconnectHandler(c)
	}

	return err
}

// read 读取消息
// 持续从流中读取消息，更新心跳时间、检测空包/心跳包并分发到接收hook；读取失败时触发强制关闭
// @param conn net.Conn TCP连接
func (c *clientConn) read(conn *websocket.Conn) {
	var index int

	for {
		mt, r, err := conn.NextReader()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				if _, ok := err.(*websocket.CloseError); !ok {
					log.Warnf("read message failed: %d %v", c.id, err)
				}
			}

			taskpool.Add(func() { c.forceClose() })
			return
		}

		if mt != websocket.BinaryMessage {
			_, _ = io.Copy(io.Discard, r)
			continue
		}

		isHeartbeat, heartbeatTime, buf, err := packet.Read(r)
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
			}
		}
	}
}

// write 写入消息
// @param conn *websocket.Conn WS连接
func (c *clientConn) write(conn *websocket.Conn) {
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

// doWrite 执行写入操作
// 识别关闭信号后终止写协程，否则写入消息数据并释放缓冲区
// @param conn *websocket.Conn WS连接
// @param buf buffer.Buffer 待写入的消息缓冲区
func (c *clientConn) doWrite(conn *websocket.Conn, buf buffer.Buffer) {
	closeSig := buf.Len() == 0

	c.queue.Done(closeSig)

	if closeSig {
		buf.Release()
		return
	}

	if c.cli.opts.writeTimeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(c.cli.opts.writeTimeout))
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, buf.Bytes()); err != nil {
		if _, ok := err.(*websocket.CloseError); !ok {
			if !errors.Is(err, net.ErrClosed) {
				log.Errorf("write message error: %v", err)
			}

			taskpool.Add(func() { c.forceClose() })
		}
	}

	buf.Release()
}

// doHandleHeartbeat 处理心跳
// 检测上次收到消息的时间是否超时，超时则触发强制关闭；否则下发心跳包
// @param conn *websocket.Conn WS连接
// @param t time.Time 当前心跳触发的时间点
// @return @1 bool 是否继续写入协程循环，心跳超时时返回false
func (c *clientConn) doHandleHeartbeat(conn *websocket.Conn, t time.Time) bool {
	if c.lastHeartbeatTime.Load() < t.Add(-2*c.cli.opts.heartbeatInterval).UnixNano() {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		taskpool.Add(func() { c.forceClose() })

		return false
	} else {
		if c.cli.opts.writeTimeout > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(c.cli.opts.writeTimeout))
		}

		hb := packet.PackHeartbeat()

		if err := conn.WriteMessage(websocket.BinaryMessage, hb.Bytes()); err != nil {
			log.Errorf("write heartbeat message error: %v", err)
		}

		hb.Release()

		return true
	}
}

// isClosed 是否已关闭
// @return @1 bool 连接状态是否为关闭
func (c *clientConn) isClosed() bool {
	return c.State() == network.ConnClosed
}
