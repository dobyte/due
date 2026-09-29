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
	wg1               sync.WaitGroup              // 读等待组，随连接对象池复用
	wg2               sync.WaitGroup              // 写等待组，随连接对象池复用
	conn              *kcp.UDPSession             // KCP源连接
	queue             *queue.Queue[buffer.Buffer] // 消息队列
	dueBuffers        []buffer.Buffer             // 待写入的消息缓冲对象集合
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
// 解绑后重新开启授权定时检测
// @return @1 error 连接已关闭时返回errors.ErrConnectionClosed
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
// 连接关闭后连接对象将被回收复用，不应再使用其任何方法与属性（身份标识可能漂移）
// @param force ...bool 是否强制关闭；为true时立即关闭，缺省或为false时执行优雅关闭
// @return @1 error 关闭失败或连接已处于关闭态时返回的错误
func (c *serverConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose()
	} else {
		return c.graceClose()
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
// 复用对象池中的连接对象，在写锁保护下完成状态重置、分片存储与读写协程启动，
// 关闭路径与上一生命周期的延迟关闭任务均需获取锁，将被阻塞至初始化完成，
// 避免连接对象在协程启动前被回收复用；服务器关闭过程中分片拒绝存储时初始化中止，
// 由调用方归还连接对象并关闭底层连接
// @param conn *kcp.UDPSession KCP连接
// @return @1 bool 是否初始化成功，服务器关闭过程中返回false
func (c *serverConn) init(conn *kcp.UDPSession) bool {
	c.rw.Lock()

	c.id = c.connMgr.cid.Add(1)
	c.uid.Store(0)
	c.attr.values.Clear()
	c.state.Store(int32(network.ConnOpened))
	c.conn = conn
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(minWriteQueueSize, c.connMgr.server.opts.writeQueueSize)), c.connMgr.server.opts.writeTimeout)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.authorizeTimer.Store((*time.Timer)(nil))

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

	if !c.connMgr.storeConn(conn, c) {
		// 分片已停止接入，复位状态并清理引用后中止初始化
		c.state.Store(int32(network.ConnClosed))
		c.conn = nil
		c.queue = nil
		c.rw.Unlock()
		return false
	}

	c.wg1.Go(func() { c.read(conn) })
	c.wg2.Go(func() { c.write(conn) })

	c.rw.Unlock()

	// 初始化完成前连接可能已被并发关闭，仅在连接仍处于打开状态时执行授权检查与连接钩子
	if c.State() != network.ConnOpened {
		return true
	}

	c.checkAuthorize(conn)

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}

	return true
}

// reset 重置连接
// 清空连接相关字段与属性，供连接对象复用
func (c *serverConn) reset() {
	c.queue = nil
	c.attr.values.Clear()
	c.dueBuffers = c.dueBuffers[:0]
	c.netBuffers = c.netBuffers[:0]
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
// 开启授权超时定时器，超时且仍未绑定用户ID时关闭连接；定时器回调会比对连接ID与连接指针，
// 避免连接被回收复用后误关闭新连接
// @param conn *kcp.UDPSession 当前KCP连接
func (c *serverConn) checkAuthorize(conn *kcp.UDPSession) {
	if c.connMgr.server.opts.authorizeTimeout > 0 {
		id := c.id

		timer := c.authorizeTimer.Swap(time.AfterFunc(c.connMgr.server.opts.authorizeTimeout, func() {
			if c.UID() != 0 {
				return
			}

			c.recycleClose(conn, id)
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
// 写入关闭信号等待写队列排空后关闭连接，便于尽量下发完已缓冲的消息；
// 配置优雅关闭超时时间后，超时未排空将直接断开底层连接以强制结束等待
// @return @1 error 连接非打开态或关闭过程中出错时返回的错误
func (c *serverConn) graceClose() error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnHanged)) {
		return errors.ErrConnectionNotOpened
	}

	c.uncheckAuthorize()

	c.rw.RLock()
	if c.conn == nil {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}
	conn := c.conn
	q := c.queue
	err := q.Write(buffer.NewBytes(nil))
	c.rw.RUnlock()

	if err == nil {
		if closeTimeout := c.connMgr.server.opts.closeTimeout; closeTimeout > 0 {
			// 排空超时后强制断开底层连接，打断写协程中可能阻塞的写操作
			timer := time.AfterFunc(closeTimeout, func() { _ = conn.Close() })
			q.Wait()
			timer.Stop()
		} else {
			q.Wait()
		}
	}

	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose()
}

// forceClose 强制关闭
// 立即切换状态为关闭并关闭连接，不等待写队列排空
// @return @1 error 连接已处于关闭态时返回的错误
func (c *serverConn) forceClose() error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	c.uncheckAuthorize()

	return c.doClose(true)
}

// recycleClose 若当前连接仍为指定连接则强制关闭
// 读/写协程错误路径经 taskpool 异步关闭连接，闭包执行时连接对象可能已被回收复用，
// 且KCP连接对象地址可能被运行时复用，因此需同时比对连接ID与KCP连接指针，避免误关闭新连接
// @param conn *kcp.UDPSession 触发关闭时的KCP连接
// @param id int64 触发关闭时的连接ID
func (c *serverConn) recycleClose(conn *kcp.UDPSession, id int64) {
	c.rw.RLock()
	match := c.id == id && c.conn == conn
	c.rw.RUnlock()

	if match {
		c.forceClose()
	}
}

// doClose 执行关闭操作
// 关闭写队列，等待读写协程退出后关闭KCP连接，触发断开hook，并将连接对象归还连接池；
// force 为 true 时先关闭KCP连接以打断写协程中可能阻塞的写操作，保证强制关闭语义
// @param force ...bool 是否强制关闭
// @return @1 error 关闭KCP连接时的错误
func (c *serverConn) doClose(force ...bool) error {
	c.rw.Lock()
	if c.conn == nil {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	c.queue.Close()
	conn := c.conn
	c.conn = nil
	c.rw.Unlock()

	var err error

	if len(force) > 0 && force[0] {
		err = conn.Close()
		c.wg2.Wait()
	} else {
		c.wg2.Wait()
		err = conn.Close()
	}

	c.wg1.Wait()

	for buf := range c.queue.Read() {
		buf.Release()
	}

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	c.connMgr.recycleConn(conn)

	return err
}

// read 读取消息
// 循环读取KCP数据，校验连接状态与心跳包，并按心跳机制响应或将有效消息交给接收hook函数处理
// @param conn *kcp.UDPSession KCP连接
func (c *serverConn) read(conn *kcp.UDPSession) {
	var (
		index = 0
		id    = c.id
	)

	for {
		isHeartbeat, heartbeatTime, buf, err := packet.Read(conn)
		if err != nil {
			taskpool.Add(func() { c.recycleClose(conn, id) })
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
			} else {
				buf.Release()
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

			c.doBatchWrite(conn, buf)
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

// doBatchWrite 批量写入消息
// 从写队列批量取出任务，收集字节切片后通过WriteBuffers一次性下发，减少系统调用次数
// @param conn *kcp.UDPSession KCP连接
// @param first buffer.Buffer 首个已取出的任务
func (c *serverConn) doBatchWrite(conn *kcp.UDPSession, first buffer.Buffer) {
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
	c.netBuffers = c.netBuffers[:0]

	for _, buf := range c.dueBuffers {
		buf.VisitBytes(func(bytes []byte) bool {
			c.netBuffers = append(c.netBuffers, bytes)
			return true
		})
	}

	if len(c.netBuffers) > 0 {
		if c.connMgr.server.opts.writeTimeout > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(c.connMgr.server.opts.writeTimeout))
		}

		if _, err := conn.WriteBuffers(c.netBuffers); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Warnf("write message error: %v", err)

				id := c.id

				taskpool.Add(func() { c.recycleClose(conn, id) })
			}
		}
	}

	for _, buf := range c.dueBuffers {
		buf.Release()
	}

	c.netBuffers = c.netBuffers[:0]
	c.dueBuffers = c.dueBuffers[:0]
}

// doHandleHeartbeat 处理心跳
// 超过心跳超时阈值则强制关闭连接，否则按心跳机制主动发送心跳包
// @param conn *kcp.UDPSession KCP连接
// @param t time.Time 当前心跳时刻
// @return @1 bool 是否继续运行（心跳超时强制关闭返回false）
func (c *serverConn) doHandleHeartbeat(conn *kcp.UDPSession, t time.Time) bool {
	if c.lastHeartbeatTime.Load() < t.Add(-2*c.connMgr.server.opts.heartbeatInterval).UnixNano() {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		id := c.id

		taskpool.Add(func() { c.recycleClose(conn, id) })

		return false
	} else {
		if c.connMgr.server.opts.heartbeatMechanism == TickHeartbeat {
			if c.connMgr.server.opts.writeTimeout > 0 {
				_ = conn.SetWriteDeadline(time.Now().Add(c.connMgr.server.opts.writeTimeout))
			}

			hb := packet.PackHeartbeat(true)

			if _, err := conn.Write(hb.Bytes()); err != nil {
				log.Warnf("write heartbeat message error: %v", err)
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
