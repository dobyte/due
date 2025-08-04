/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/27 5:03 下午
 * @Desc: TODO
 */

package ws

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xnet"
	"github.com/dobyte/due/v2/utils/xtime"
	"github.com/gorilla/websocket"
)

type serverConn struct {
	rw                sync.RWMutex    // 锁
	id                int64           // 连接ID
	uid               int64           // 用户ID
	attr              *attr           // 连接属性
	state             int32           // 连接状态
	conn              *websocket.Conn // WS源连接
	connMgr           *serverConnMgr  // 连接管理
	chLowWrite        chan chWrite    // 低级队列
	chHighWrite       chan chWrite    // 优先队列
	done              chan struct{}   // 写入完成信号
	close             chan struct{}   // 关闭信号
	lastHeartbeatTime int64           // 上次心跳时间
}

var _ network.Conn = &serverConn{}

// ID 获取连接ID
func (c *serverConn) ID() int64 {
	return c.id
}

// UID 获取用户ID
func (c *serverConn) UID() int64 {
	return atomic.LoadInt64(&c.uid)
}

// Attr 获取属性接口
func (c *serverConn) Attr() network.Attr {
	return c.attr
}

// Bind 绑定用户ID
func (c *serverConn) Bind(uid int64) {
	atomic.StoreInt64(&c.uid, uid)
}

// Unbind 解绑用户ID
func (c *serverConn) Unbind() {
	atomic.StoreInt64(&c.uid, 0)
}

// Send 发送消息（同步）
func (c *serverConn) Send(msg []byte) (err error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err = c.checkState(); err != nil {
		return
	}

	c.chHighWrite <- chWrite{typ: dataPacket, msg: msg}

	return
}

// Push 发送消息（异步）
func (c *serverConn) Push(msg []byte) (err error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err = c.checkState(); err != nil {
		return
	}

	c.chLowWrite <- chWrite{typ: dataPacket, msg: msg}

	return
}

// State 获取连接状态
func (c *serverConn) State() network.ConnState {
	return network.ConnState(atomic.LoadInt32(&c.state))
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
	if err := c.checkState(); err != nil {
		return nil, err
	}

	c.rw.RLock()
	conn := c.conn
	c.rw.RUnlock()

	if conn == nil {
		return nil, errors.ErrConnectionClosed
	}

	return conn.LocalAddr(), nil
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
	if err := c.checkState(); err != nil {
		return nil, err
	}

	c.rw.RLock()
	conn := c.conn
	c.rw.RUnlock()

	if conn == nil {
		return nil, errors.ErrConnectionClosed
	}

	return conn.RemoteAddr(), nil
}

// 初始化连接
func (c *serverConn) init(cm *serverConnMgr, id int64, conn *websocket.Conn) {
	c.id = id
	c.attr = &attr{}
	c.conn = conn
	c.connMgr = cm
	c.chLowWrite = make(chan chWrite, 4096)
	c.chHighWrite = make(chan chWrite, 1024)
	c.done = make(chan struct{})
	c.close = make(chan struct{})
	c.lastHeartbeatTime = xtime.Now().UnixNano()
	atomic.StoreInt64(&c.uid, 0)
	atomic.StoreInt32(&c.state, int32(network.ConnOpened))

	xcall.Go(c.read)

	xcall.Go(c.write)

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}
}

// 重置连接
func (c *serverConn) reset() {
	c.attr = nil
}

// 检测连接状态
func (c *serverConn) checkState() error {
	switch network.ConnState(atomic.LoadInt32(&c.state)) {
	case network.ConnHanged:
		return errors.ErrConnectionHanged
	case network.ConnClosed:
		return errors.ErrConnectionClosed
	default:
		return nil
	}
}

// 优雅关闭
func (c *serverConn) graceClose(isNeedRecycle bool) error {
	if !atomic.CompareAndSwapInt32(&c.state, int32(network.ConnOpened), int32(network.ConnHanged)) {
		return errors.ErrConnectionNotOpened
	}

	c.rw.RLock()
	c.chLowWrite <- chWrite{typ: closeSig}
	c.rw.RUnlock()

	<-c.done

	if !atomic.CompareAndSwapInt32(&c.state, int32(network.ConnHanged), int32(network.ConnClosed)) {
		return errors.ErrConnectionNotHanged
	}

	c.rw.Lock()
	close(c.chLowWrite)
	close(c.chHighWrite)
	close(c.close)
	close(c.done)
	conn := c.conn
	c.conn = nil
	c.rw.Unlock()

	err := conn.Close()

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	if isNeedRecycle {
		c.connMgr.recycle(conn)
	}

	return err
}

// 强制关闭
func (c *serverConn) forceClose(isNeedRecycle bool) error {
	if !atomic.CompareAndSwapInt32(&c.state, int32(network.ConnOpened), int32(network.ConnClosed)) {
		if !atomic.CompareAndSwapInt32(&c.state, int32(network.ConnHanged), int32(network.ConnClosed)) {
			return errors.ErrConnectionClosed
		}
	}

	c.rw.Lock()
	close(c.chLowWrite)
	close(c.chHighWrite)
	close(c.close)
	close(c.done)
	conn := c.conn
	c.conn = nil
	c.rw.Unlock()

	err := conn.Close()

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	if isNeedRecycle {
		c.connMgr.recycle(conn)
	}

	return err
}

// 读取消息
func (c *serverConn) read() {
	conn := c.conn

	for {
		select {
		case <-c.close:
			return
		default:
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					if _, ok := err.(*websocket.CloseError); !ok {
						log.Warnf("read message failed: %d %v", c.id, err)
					}
				}
				_ = c.forceClose(true)
				return
			}

			if msgType != websocket.BinaryMessage {
				continue
			}

			if c.connMgr.server.opts.heartbeatInterval > 0 {
				atomic.StoreInt64(&c.lastHeartbeatTime, xtime.Now().UnixNano())
			}

			switch c.State() {
			case network.ConnHanged:
				continue
			case network.ConnClosed:
				return
			default:
				// ignore
			}

			// ignore empty packet
			if len(msg) == 0 {
				continue
			}

			// check heartbeat packet
			isHeartbeat, err := packet.CheckHeartbeat(msg)
			if err != nil {
				log.Errorf("check heartbeat message error: %v", err)
				continue
			}

			// ignore heartbeat packet
			if isHeartbeat {
				// responsive heartbeat
				if c.connMgr.server.opts.heartbeatMechanism == RespHeartbeat {
					c.rw.RLock()
					c.chHighWrite <- chWrite{typ: heartbeatPacket}
					c.rw.RUnlock()
				}
				continue
			}

			if c.connMgr.server.receiveHandler != nil {
				c.connMgr.server.receiveHandler(c, msg)
			}
		}
	}
}

// 写入消息
// 由于gorilla/websocket库并发写入的限制，同时为了保证心跳能够优先下发到客户端，故而实现一个优先队列
func (c *serverConn) write() {
	var (
		conn   = c.conn
		ticker *time.Ticker
	)

	if c.connMgr.server.opts.heartbeatInterval > 0 {
		ticker = time.NewTicker(c.connMgr.server.opts.heartbeatInterval)
		defer ticker.Stop()
	} else {
		ticker = &time.Ticker{C: make(chan time.Time, 1)}
	}

	for {
		select {
		case r, ok := <-c.chHighWrite:
			if !ok {
				return
			}

			if !c.doWrite(conn, r) {
				return
			}
		case <-ticker.C:
			if !c.doHandleHeartbeat(conn) {
				return
			}
		default:
			select {
			case r, ok := <-c.chHighWrite:
				if !ok {
					return
				}

				if !c.doWrite(conn, r) {
					return
				}
			case r, ok := <-c.chLowWrite:
				if !ok {
					return
				}

				if !c.doWrite(conn, r) {
					return
				}
			case <-ticker.C:
				if !c.doHandleHeartbeat(conn) {
					return
				}
			}
		}
	}
}

// 执行写入操作
func (c *serverConn) doWrite(conn *websocket.Conn, r chWrite) bool {
	if r.typ == closeSig {
		c.rw.RLock()
		c.done <- struct{}{}
		c.rw.RUnlock()
		return false
	}

	if c.isClosed() {
		return false
	}

	if r.typ == heartbeatPacket {
		if msg, err := packet.PackHeartbeat(); err != nil {
			log.Errorf("pack heartbeat message error: %v", err)
			return true
		} else {
			r.msg = msg
		}
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, r.msg); err != nil {
		if !errors.Is(err, net.ErrClosed) {
			if _, ok := err.(*websocket.CloseError); !ok {
				log.Errorf("write message error: %v", err)
			}
		}
	}

	return true
}

// 处理心跳
func (c *serverConn) doHandleHeartbeat(conn *websocket.Conn) bool {
	deadline := xtime.Now().Add(-2 * c.connMgr.server.opts.heartbeatInterval).UnixNano()
	if atomic.LoadInt64(&c.lastHeartbeatTime) < deadline {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)
		_ = c.forceClose(true)
		return false
	} else {
		if c.connMgr.server.opts.heartbeatMechanism == TickHeartbeat {
			if c.isClosed() {
				return false
			}

			if heartbeat, err := packet.PackHeartbeat(); err != nil {
				log.Errorf("pack heartbeat message error: %v", err)
			} else {
				// send heartbeat packet
				if err := conn.WriteMessage(websocket.BinaryMessage, heartbeat); err != nil {
					log.Errorf("write heartbeat message error: %v", err)
				}
			}
		}
	}

	return true
}

// 是否已关闭
func (c *serverConn) isClosed() bool {
	return network.ConnState(atomic.LoadInt32(&c.state)) == network.ConnClosed
}
