package kcp

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
	"github.com/xtaci/kcp-go/v5"
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
	conn              *kcp.UDPSession             // KCP源连接
	queue             *queue.Queue[buffer.Buffer] // 消息队列
	netBuffers        net.Buffers                 // 待写入的字节切片集合
	lastHeartbeatTime atomic.Int64                // 上次心跳时间
	authorizeTimer    atomic.Value                // 授权定时器
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
// 绑定成功后取消授权定时检测
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
// 解绑后重新开启授权定时检测
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

// Push 发送消息
// 消息写入写队列，由写协程统一下发
// @param buf buffer.Buffer 消息内容，消息发送失败自行控制释放buffer
// @return @1 error 连接状态异常或队列写入失败时返回的错误
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

	conn := c.conn
	c.rw.RUnlock()

	return conn.LocalAddr(), nil
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

	conn := c.conn
	c.rw.RUnlock()

	return conn.RemoteAddr(), nil
}

// init 初始化连接
// 复用连接对象，重置状态、写队列与两路读写协程，并应用服务器相关KCP参数
// @param conn *kcp.UDPSession KCP连接
func (c *serverConn) init(conn *kcp.UDPSession) {
	c.id = c.connMgr.genConnID()
	c.uid.Store(0)
	c.attr.values.Clear()
	c.state.Store(int32(network.ConnOpened))
	c.conn = conn
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(128, c.connMgr.server.opts.writeQueueSize)), c.connMgr.server.opts.writeTimeout)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.authorizeTimer.Store((*time.Timer)(nil))
	c.connMgr.storeConn(conn, c)
	c.wg1 = &sync.WaitGroup{}
	c.wg1.Go(func() { c.read(conn) })
	c.wg2 = &sync.WaitGroup{}
	c.wg2.Go(func() { c.write(conn) })

	if c.connMgr.server.opts.mtu > 0 {
		conn.SetMtu(c.connMgr.server.opts.mtu)
	}

	if len(c.connMgr.server.opts.noDelay) == 4 {
		conn.SetNoDelay(c.connMgr.server.opts.noDelay[0], c.connMgr.server.opts.noDelay[1], c.connMgr.server.opts.noDelay[2], c.connMgr.server.opts.noDelay[3])
	}

	if c.connMgr.server.opts.ackNoDelay {
		conn.SetACKNoDelay(c.connMgr.server.opts.ackNoDelay)
	}

	if c.connMgr.server.opts.writeDelay {
		conn.SetWriteDelay(c.connMgr.server.opts.writeDelay)
	}

	if len(c.connMgr.server.opts.windowSize) == 2 {
		conn.SetWindowSize(c.connMgr.server.opts.windowSize[0], c.connMgr.server.opts.windowSize[1])
	}

	if c.connMgr.server.opts.readBuffer > 0 {
		conn.SetReadBuffer(c.connMgr.server.opts.readBuffer)
	}

	if c.connMgr.server.opts.writeBuffer > 0 {
		conn.SetWriteBuffer(c.connMgr.server.opts.writeBuffer)
	}

	c.checkAuthorize()

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}
}

// reset 重置连接
// 清空连接相关字段与属性，供连接对象复用
func (c *serverConn) reset() {
	c.wg1 = nil
	c.wg2 = nil
	c.conn = nil
	c.queue = nil
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
// 在授权超时后若仍未绑定用户ID，则强制关闭连接
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
// 停止并清空授权定时器，解除强制关闭的约束
func (c *serverConn) uncheckAuthorize() {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		timer := c.authorizeTimer.Swap((*time.Timer)(nil))

		if t, ok := timer.(*time.Timer); ok && t != nil {
			t.Stop()
		}
	}
}

// graceClose 优雅关闭
// 向写队列写入关闭信号，等待队列排空后关闭连接，便于尽量下发完已缓冲的消息
// @param isNeedRecycle bool 关闭后是否需要回收连接对象
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
// 直接切换连接状态为关闭并执行关闭操作，不等待队列排空
// @param isNeedRecycle bool 关闭后是否需要回收连接对象
// @return @1 error 连接已处于关闭态或关闭过程中出错时返回的错误
func (c *serverConn) forceClose(isNeedRecycle bool) error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	c.uncheckAuthorize()

	return c.doClose(isNeedRecycle)
}

// doClose 执行关闭操作
// 关闭写队列，等待写协程退出后关闭底层连接，触发断开hook函数并按需回收连接
// @param isNeedRecycle bool 关闭后是否需要回收连接对象
// @return @1 error 连接已关闭或关闭底层连接失败时返回的错误
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

// read 读取消息
// 循环读取KCP数据，校验连接状态与心跳包，并按心跳机制响应或将有效消息交给接收hook函数处理
// @param conn *kcp.UDPSession KCP连接
func (c *serverConn) read(conn *kcp.UDPSession) {
	var index = 0

	for {
		isHeartbeat, heartbeatTime, buf, err := packet.Read(conn)
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

			// trigger heartbeat handler
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
// 从写队列取出消息写入连接，并按心跳间隔触发心跳检测与下发
// @param conn *kcp.UDPSession KCP连接
func (c *serverConn) write(conn *kcp.UDPSession) {
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
// 判断是否为关闭信号，否则将消息字节写入底层连接并释放缓冲区
// @param conn *kcp.UDPSession KCP连接
// @param buf buffer.Buffer 待写入的消息缓冲
func (c *serverConn) doWrite(conn *kcp.UDPSession, buf buffer.Buffer) {
	closeSig := buf.Len() == 0

	c.queue.Done(closeSig)

	if closeSig {
		buf.Release()
		return
	}

	var err error

	switch n := buf.Nodes(); n {
	case 0:
		// ignore
	case 1:
		_, err = conn.Write(buf.Bytes())
	case 2:
		buf.VisitBytes(func(b []byte) bool {
			c.netBuffers = append(c.netBuffers, b)
			return true
		})

		_, err = conn.WriteBuffers(c.netBuffers)

		c.netBuffers = c.netBuffers[:0]
	}

	if err != nil && !errors.Is(err, net.ErrClosed) {
		log.Errorf("write message error: %v", err)
		taskpool.Add(func() { c.forceClose(true) })
	}

	buf.Release()
}

// doHandleHeartbeat 处理心跳
// 超过心跳超时阈值则强制关闭连接，否则按心跳机制主动发送心跳包
// @param conn *kcp.UDPSession KCP连接
// @param t time.Time 当前心跳时刻
// @return @1 bool 是否继续运行（心跳超时强制关闭返回false）
func (c *serverConn) doHandleHeartbeat(conn *kcp.UDPSession, t time.Time) bool {
	if c.lastHeartbeatTime.Load() < t.Add(-2*c.connMgr.server.opts.heartbeatInterval).UnixNano() {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		taskpool.Add(func() { c.forceClose(true) })

		return false
	} else {
		if c.connMgr.server.opts.heartbeatMechanism == TickHeartbeat {
			hb := packet.PackHeartbeat(true)

			if _, err := conn.Write(hb.Bytes()); err != nil {
				log.Errorf("write heartbeat message error: %v", err)
			}

			hb.Release()
		}
	}

	return true
}

// isClosed 是否已关闭
// @return @1 bool 连接状态是否为关闭
func (c *serverConn) isClosed() bool {
	return c.State() == network.ConnClosed
}
