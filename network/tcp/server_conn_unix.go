//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux
// +build darwin netbsd freebsd openbsd dragonfly linux

package tcp

import (
	"github.com/cloudwego/netpoll"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xnet"
	"github.com/dobyte/due/v2/utils/xtime"
	"net"
	"sync/atomic"
)

type serverConn struct {
	id                int64              // 连接ID
	uid               int64              // 用户ID
	state             int32              // 连接状态
	conn              netpoll.Connection // 源连接
	connMgr           *serverConnMgr     // 连接管理
	lastHeartbeatTime int64              // 上次心跳时间
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

// Bind 绑定用户ID
func (c *serverConn) Bind(uid int64) {
	atomic.StoreInt64(&c.uid, uid)
}

// Unbind 解绑用户ID
func (c *serverConn) Unbind() {
	atomic.StoreInt64(&c.uid, 0)
}

// Send 发送消息（同步）
func (c *serverConn) Send(msg []byte) error {
	if err := c.checkState(); err != nil {
		return err
	}

	conn := c.conn

	if conn == nil {
		return errors.ErrConnectionClosed
	}

	return write(conn.Writer(), msg)
}

// Push 发送消息（异步）
func (c *serverConn) Push(msg []byte) error {
	if err := c.checkState(); err != nil {
		return err
	}

	conn := c.conn

	if conn == nil {
		return errors.ErrConnectionClosed
	}

	return write(conn.Writer(), msg)
}

// State 获取连接状态
func (c *serverConn) State() network.ConnState {
	return network.ConnState(atomic.LoadInt32(&c.state))
}

// Close 关闭连接
func (c *serverConn) Close(isForce ...bool) error {
	return c.close(true)
}

// 关闭连接
func (c *serverConn) close(isNeedRecycle bool) error {
	if !atomic.CompareAndSwapInt32(&c.state, int32(network.ConnOpened), int32(network.ConnClosed)) {
		return errors.ErrConnectionClosed
	}

	err := c.conn.Close()

	if isNeedRecycle {
		c.connMgr.recycle(c.conn)
	}

	c.conn = nil

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	return err
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

	conn := c.conn

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

	conn := c.conn

	if conn == nil {
		return nil, errors.ErrConnectionClosed
	}

	return conn.RemoteAddr(), nil
}

// 初始化连接
func (c *serverConn) init(id int64, conn netpoll.Connection, cm *serverConnMgr) {
	c.id = id
	c.conn = conn
	c.connMgr = cm
	c.lastHeartbeatTime = xtime.Now().Unix()
	atomic.StoreInt64(&c.uid, 0)
	atomic.StoreInt32(&c.state, int32(network.ConnOpened))
	_ = conn.AddCloseCallback(func(_ netpoll.Connection) error { return c.Close() })

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}
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

// 读取消息
func (c *serverConn) read() error {
	if c.isClosed() {
		return errors.ErrConnectionClosed
	}

	conn := c.conn

	if conn == nil {
		return errors.ErrConnectionClosed
	}

	reader := conn.Reader()

	// block reading messages from the client
	msg, err := packet.ReadMessage(reader)
	if err != nil {
		return err
	}

	if err = reader.Release(); err != nil {
		log.Errorf("release failed: %v", err)
		return nil
	}

	// ignore empty packet
	if len(msg) == 0 {
		return nil
	}

	// check heartbeat packet
	isHeartbeat, err := packet.CheckHeartbeat(msg)
	if err != nil {
		log.Errorf("check heartbeat message error: %v", err)
		return nil
	}

	if c.connMgr.server.opts.heartbeatInterval > 0 {
		atomic.StoreInt64(&c.lastHeartbeatTime, xtime.Now().Unix())
	}

	if isHeartbeat {
		// responsive heartbeat
		if c.connMgr.server.opts.heartbeatMechanism == RespHeartbeat {
			if heartbeat, err := packet.PackHeartbeat(); err != nil {
				log.Errorf("pack heartbeat message error: %v", err)
			} else {
				if err = write(conn.Writer(), heartbeat); err != nil {
					log.Errorf("write heartbeat message error: %v", err)
				}
			}
		}
	} else {
		if c.connMgr.server.receiveHandler != nil {
			c.connMgr.server.receiveHandler(c, msg)
		}
	}

	return nil
}

// 执行心跳
func (c *serverConn) heartbeat() error {
	if c.isClosed() {
		return errors.ErrConnectionClosed
	}

	conn := c.conn

	if conn == nil {
		return errors.ErrConnectionClosed
	}

	heartbeat, err := packet.PackHeartbeat()
	if err != nil {
		return err
	}

	return write(conn.Writer(), heartbeat)
}

// 是否已关闭
func (c *serverConn) isClosed() bool {
	return network.ConnState(atomic.LoadInt32(&c.state)) == network.ConnClosed
}
