/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/27 5:03 下午
 * @Desc: TODO
 */

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

type serverConn struct {
	id                int64                       // 连接ID
	uid               atomic.Int64                // 用户ID
	attr              *attr                       // 连接属性
	state             atomic.Int32                // 连接状态
	connMgr           *serverConnMgr              // 连接管理
	rw                sync.RWMutex                // 锁
	wg1               *sync.WaitGroup             // 读等待组
	wg2               *sync.WaitGroup             // 写等待组
	conn              *websocket.Conn             // WS源连接
	remoteAddr        net.Addr                    // 客户端真实地址（应用层代理模式下从代理头解析）
	queue             *queue.Queue[buffer.Buffer] // 消息队列
	lastHeartbeatTime atomic.Int64                // 上次心跳时间
	authorizeTimer    atomic.Pointer[time.Timer]  // 授权定时器
}

var _ network.Conn = &serverConn{}

// ID 获取连接ID
// @return @1 int64 连接ID
func (c *serverConn) ID() int64 {
	return c.id
}

// UID 获取用户ID
// @return @1 int64 用户ID
func (c *serverConn) UID() int64 {
	return c.uid.Load()
}

// Attr 获取属性接口
// @return @1 network.Attr 属性接口
func (c *serverConn) Attr() network.Attr {
	return c.attr
}

// Bind 绑定用户ID
// @param uid int64 用户ID
// @return @1 error 错误信息
func (c *serverConn) Bind(uid int64) error {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}

	c.uid.Store(uid)
	c.uncheckAuthorize()

	c.rw.RUnlock()

	return nil
}

// Unbind 解绑用户ID
// @return @1 error 错误信息
func (c *serverConn) Unbind() error {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}

	c.uid.Store(0)
	c.checkAuthorize(c.conn)

	c.rw.RUnlock()

	return nil
}

// Push 发送消息
// @param buf buffer.Buffer 消息内容，消息发送失败自行控制释放buffer
// @return @1 error 错误信息
func (c *serverConn) Push(buf buffer.Buffer) error {
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
func (c *serverConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close 关闭连接
// @param force ...bool 是否强制关闭
// @return @1 error 错误信息
func (c *serverConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose(true)
	} else {
		return c.graceClose(true)
	}
}

// LocalIP 获取本地IP
// @return @1 string 本地IP地址
// @return @2 error 错误信息
func (c *serverConn) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr 获取本地地址
// @return @1 net.Addr 本地地址
// @return @2 error 错误信息
func (c *serverConn) LocalAddr() (net.Addr, error) {
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
func (c *serverConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr 获取远端地址
// @return @1 net.Addr 远端地址
// @return @2 error 错误信息
func (c *serverConn) RemoteAddr() (net.Addr, error) {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return nil, errors.ErrConnectionClosed
	}

	conn := c.conn
	remoteAddr := c.remoteAddr
	c.rw.RUnlock()

	if remoteAddr != nil {
		return remoteAddr, nil
	}

	return conn.RemoteAddr(), nil
}

// init 初始化连接
// 复用对象池中的连接对象，重置各项状态、创建读写协程并执行授权检查与连接钩子
// @param conn *websocket.Conn WS连接
// @param remoteAddr net.Addr 客户端真实地址，可为nil
func (c *serverConn) init(conn *websocket.Conn, remoteAddr net.Addr) {
	c.id = c.connMgr.genConnID()
	c.uid.Store(0)
	c.attr.values.Clear()
	c.state.Store(int32(network.ConnOpened))
	c.conn = conn
	c.remoteAddr = remoteAddr
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(128, c.connMgr.server.opts.writeQueueSize)), c.connMgr.server.opts.writeTimeout)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.authorizeTimer.Store(nil)
	c.connMgr.storeConn(conn, c)
	c.wg1 = &sync.WaitGroup{}
	c.wg1.Go(func() { c.read(conn) })
	c.wg2 = &sync.WaitGroup{}
	c.wg2.Go(func() { c.write(conn) })

	c.checkAuthorize(conn)

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}
}

// reset 重置连接
// 清空连接对象内的引用与状态，以便归还对象池后安全复用；
// 并显式停止残留的授权定时器，避免其回调在连接复用后触发
func (c *serverConn) reset() {
	c.wg1 = nil
	c.wg2 = nil
	c.conn = nil
	c.remoteAddr = nil
	c.queue = nil
	c.attr.values.Clear()
	c.uncheckAuthorize()
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
// 开启授权超时定时器，超时且仍未绑定用户ID时关闭连接；定时器回调会比对连接指针，
// 避免连接被回收复用后误关闭新连接
// @param conn *websocket.Conn 当前WS连接
func (c *serverConn) checkAuthorize(conn *websocket.Conn) {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		if timer := c.authorizeTimer.Swap(time.AfterFunc(c.connMgr.server.opts.authorizeTimeout, func() {
			if c.UID() != 0 {
				return
			}

			c.recycleClose(conn)
		})); timer != nil {
			timer.Stop()
		}
	}
}

// uncheckAuthorize 取消授权检查
// 停止授权超时定时器，用于绑定用户ID或关闭连接时解除授权检测
func (c *serverConn) uncheckAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		if timer := c.authorizeTimer.Swap(nil); timer != nil {
			timer.Stop()
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
// 关闭写队列，等待读写协程退出后关闭WS连接，触发断开hook，并按需归还连接对象
// @param isNeedRecycle bool 是否在关闭后将连接对象归还连接池
// @return @1 error 关闭WS连接时的错误
func (c *serverConn) doClose(isNeedRecycle bool) error {
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

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	if isNeedRecycle {
		c.connMgr.recycleConn(conn)
	}

	return err
}

// recycleClose 若当前连接仍为指定连接则强制关闭
// 读/写协程错误路径经 taskpool 异步关闭连接，闭包执行时连接可能已被回收复用，
// 因此需比对 WS 连接指针，避免误关闭新连接
// @param conn *websocket.Conn 触发关闭时的WS连接
func (c *serverConn) recycleClose(conn *websocket.Conn) {
	c.rw.RLock()
	match := c.conn == conn
	c.rw.RUnlock()

	if match {
		c.forceClose(true)
	}
}

// read 读取消息
// 持续读取消息，更新心跳时间、检测空包/心跳包并分发到接收hook；读取失败时触发强制关闭
// @param conn *websocket.Conn WS连接
func (c *serverConn) read(conn *websocket.Conn) {
	var index int

	for {
		mt, r, err := conn.NextReader()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				if _, ok := err.(*websocket.CloseError); !ok {
					log.Warnf("read message failed: %d %v", c.id, err)
				}
			}

			taskpool.Add(func() { c.recycleClose(conn) })
			return
		}

		if mt != websocket.BinaryMessage {
			_, _ = io.Copy(io.Discard, r)
			continue
		}

		isHeartbeat, heartbeatTime, buf, err := packet.Read(r)
		if err != nil {
			taskpool.Add(func() { c.recycleClose(conn) })
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
			if c.connMgr.server.opts.heartbeatInterval > 0 {
				c.lastHeartbeatTime.Store(time.Now().UnixNano())
			}

			// responsive heartbeat
			if c.connMgr.server.opts.heartbeatMechanism == RespHeartbeat {
				hb := packet.PackHeartbeat(true)

				c.rw.RLock()
				err := c.queue.Write(hb)
				c.rw.RUnlock()

				if err != nil {
					hb.Release()
				}
			}

			// trigger heartbeat handler, heartbeat time is not nil
			if c.connMgr.server.heartbeatHandler != nil {
				c.connMgr.server.heartbeatHandler(c, heartbeatTime)
			}
		} else {
			// update heartbeat time
			if c.connMgr.server.opts.heartbeatInterval > 0 {
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

			if c.connMgr.server.receiveHandler != nil {
				c.connMgr.server.receiveHandler(c, buf)
			}
		}
	}
}

// write 写入消息
// @param conn *websocket.Conn WS连接
func (c *serverConn) write(conn *websocket.Conn) {
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
func (c *serverConn) doWrite(conn *websocket.Conn, buf buffer.Buffer) {
	closeSig := buf.Len() == 0

	c.queue.Done(closeSig)

	if closeSig {
		buf.Release()
		return
	}

	if c.connMgr.server.opts.writeTimeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(c.connMgr.server.opts.writeTimeout))
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, buf.Bytes()); err != nil {
		if errors.Is(err, net.ErrClosed) {
			taskpool.Add(func() { c.recycleClose(conn) })
		} else {
			if _, ok := err.(*websocket.CloseError); !ok {
				log.Errorf("write message error: %v", err)
			}
		}
	}

	buf.Release()
}

// doHandleHeartbeat 处理心跳
// 检测上次收到消息的时间是否超时，超时则触发强制关闭；主动定时心跳模式下额外下发心跳包
// @param conn *websocket.Conn WS连接
// @param t time.Time 当前心跳触发的时间点
// @return @1 bool 是否继续写入协程循环，心跳超时时返回false
func (c *serverConn) doHandleHeartbeat(conn *websocket.Conn, t time.Time) bool {
	if c.lastHeartbeatTime.Load() < t.Add(-2*c.connMgr.server.opts.heartbeatInterval).UnixNano() {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		taskpool.Add(func() { c.recycleClose(conn) })

		return false
	} else {
		if c.connMgr.server.opts.heartbeatMechanism == TickHeartbeat {
			if c.connMgr.server.opts.writeTimeout > 0 {
				_ = conn.SetWriteDeadline(time.Now().Add(c.connMgr.server.opts.writeTimeout))
			}

			hb := packet.PackHeartbeat(true)

			if err := conn.WriteMessage(websocket.BinaryMessage, hb.Bytes()); err != nil {
				log.Errorf("write heartbeat message error: %v", err)
			}

			hb.Release()
		}

		return true
	}
}

// isClosed 是否已关闭
// @return @1 bool 连接状态是否为关闭
func (c *serverConn) isClosed() bool {
	return c.State() == network.ConnClosed
}
