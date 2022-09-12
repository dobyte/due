/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/27 5:03 下午
 * @Desc: TODO
 */

package ws

import (
	"github.com/dobyte/due/internal/xnet"
	"github.com/dobyte/due/log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/dobyte/due/network"
)

type serverConn struct {
	rw                sync.RWMutex    // 锁
	id                int64           // 连接ID
	uid               int64           // 用户ID
	state             int32           // 连接状态
	conn              *websocket.Conn // WS源连接
	connMgr           *connMgr        // 连接管理
	chWrite           chan chWrite    // 写入队列
	done              chan struct{}   // 写入完成信号
	lastHeartbeatTime int64           // 上次心跳时间
}

var _ network.Conn = &serverConn{}

// ID 获取连接ID
func (c *serverConn) ID() int64 {
	return c.id
}

// UID 获取用户ID
func (c *serverConn) UID() int64 {
	c.rw.RLock()
	defer c.rw.RUnlock()

	return c.uid
}

// Bind 绑定用户ID
func (c *serverConn) Bind(uid int64) {
	c.rw.Lock()
	defer c.rw.Unlock()

	c.uid = uid
}

// Send 发送消息（同步）
func (c *serverConn) Send(msg []byte, msgType ...int) error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	if len(msgType) == 0 {
		msgType = append(msgType, TextMessage)
	}

	switch msgType[0] {
	case TextMessage, BinaryMessage:
		return c.conn.WriteMessage(msgType[0], msg)
	default:
		return network.ErrIllegalMsgType
	}
}

// Push 发送消息（异步）
func (c *serverConn) Push(msg []byte, msgType ...int) error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	if len(msgType) == 0 {
		msgType = append(msgType, TextMessage)
	}

	switch msgType[0] {
	case TextMessage, BinaryMessage:
		c.chWrite <- chWrite{typ: dataPacket, msg: msg, msgType: msgType[0]}
	default:
		return network.ErrIllegalMsgType
	}

	return nil
}

// State 获取连接状态
func (c *serverConn) State() network.ConnState {
	return network.ConnState(atomic.LoadInt32(&c.state))
}

// Close 关闭连接（主动关闭）
func (c *serverConn) Close(isForce ...bool) error {
	c.rw.Lock()
	defer c.rw.Unlock()

	if err := c.checkState(); err != nil {
		return err
	}

	if len(isForce) > 0 && isForce[0] {
		atomic.StoreInt32(&c.state, int32(network.ConnClosed))
	} else {
		atomic.StoreInt32(&c.state, int32(network.ConnHanged))
		c.chWrite <- chWrite{typ: closeSig}
		<-c.done
	}

	close(c.chWrite)

	if err := c.conn.Close(); err != nil {
		return err
	}
	c.conn = nil
	c.connMgr.recycle(c)

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	return nil
}

// 关闭连接（被动关闭）
func (c *serverConn) close() {
	c.rw.Lock()
	defer c.rw.Unlock()

	if err := c.checkState(); err != nil {
		return
	}

	atomic.StoreInt32(&c.state, int32(network.ConnClosed))

	close(c.chWrite)

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	c.conn = nil
	c.connMgr.recycle(c)
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
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return nil, err
	}

	return c.conn.LocalAddr(), nil
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
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return nil, err
	}

	return c.conn.RemoteAddr(), nil
}

// 初始化连接
func (c *serverConn) init(conn *websocket.Conn, cm *connMgr) {
	c.id = cm.id
	c.conn = conn
	c.connMgr = cm
	c.chWrite = make(chan chWrite, 256)
	c.done = make(chan struct{})
	atomic.StoreInt64(&c.lastHeartbeatTime, time.Now().Unix())
	atomic.StoreInt32(&c.state, int32(network.ConnOpened))

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}

	go c.read()

	go c.write()
}

// 检测连接状态
func (c *serverConn) checkState() error {
	switch network.ConnState(atomic.LoadInt32(&c.state)) {
	case network.ConnHanged:
		return network.ErrConnectionHanged
	case network.ConnClosed:
		return network.ErrConnectionClosed
	}

	return nil
}

// 读取消息
func (c *serverConn) read() {
	for {
		msgType, msg, err := c.conn.ReadMessage()
		if err != nil {
			c.close()
			return
		}

		atomic.StoreInt64(&c.lastHeartbeatTime, time.Now().Unix())

		switch c.State() {
		case network.ConnHanged:
			continue
		case network.ConnClosed:
			return
		}

		// ignore heartbeat packet
		if len(msg) == 0 {
			continue
		}

		if c.connMgr.server.receiveHandler != nil {
			c.connMgr.server.receiveHandler(c, msg, msgType)
		}
	}
}

// 写入消息
func (c *serverConn) write() {
	ticker := time.NewTicker(c.connMgr.server.opts.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case write, ok := <-c.chWrite:
			if !ok {
				return
			}

			if write.typ == closeSig {
				c.done <- struct{}{}
				return
			}

			if err := c.conn.WriteMessage(write.msgType, write.msg); err != nil {
				log.Errorf("write message error: %v", err)
			}
		case <-ticker.C:
			deadline := time.Now().Add(-2 * c.connMgr.server.opts.heartbeatInterval).Unix()
			if atomic.LoadInt64(&c.lastHeartbeatTime) < deadline {
				log.Warnf("connection heartbeat timeout")
				_ = c.Close(true)
				return
			}
		}
	}
}
