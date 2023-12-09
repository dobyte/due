package kcp

import (
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/network"
	"github.com/symsimmy/due/utils/xnet"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type serverConn struct {
	rw                sync.RWMutex      // 锁
	id                int64             // 连接ID
	uid               int64             // 用户ID
	state             network.ConnState // 连接状态
	conn              net.Conn          // UDP源连接
	connMgr           *serverConnMgr    // 连接管理
	chWrite           chan chWrite      // 写入队列
	lastHeartbeatTime int64             // 上次心跳时间
	done              chan struct{}     // 写入完成信号
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
func (c *serverConn) Send(msg []byte, msgType ...int) (err error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err = c.checkState(); err != nil {
		return
	}

	_, err = c.conn.Write(msg)

	return
}

// Push 发送消息（异步）
func (c *serverConn) Push(msg []byte, msgType ...int) (err error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err = c.checkState(); err != nil {
		return
	}

	c.chWrite <- chWrite{typ: dataPacket, msg: msg}

	return
}

// State 获取连接状态
func (c *serverConn) State() network.ConnState {
	c.rw.RLock()
	defer c.rw.RUnlock()

	return c.state
}

// Close 关闭连接
func (c *serverConn) Close(isForce ...bool) error {
	if len(isForce) > 0 && isForce[0] {
		return c.forceClose()
	} else {
		return c.graceClose()
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
func (c *serverConn) init(conn net.Conn, cm *serverConnMgr) {
	c.id = cm.id
	c.conn = conn
	c.connMgr = cm
	c.state = network.ConnOpened
	c.chWrite = make(chan chWrite, 256)
	c.done = make(chan struct{})
	c.lastHeartbeatTime = time.Now().Unix()

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}

	go c.read()

	go c.write()
}

// 检测连接状态
func (c *serverConn) checkState() error {
	switch c.state {
	case network.ConnHanged:
		return network.ErrConnectionHanged
	case network.ConnClosed:
		return network.ErrConnectionClosed
	}

	return nil
}

// 读取消息
func (c *serverConn) read() {
	size := c.connMgr.server.opts.maxMsgLen + 1
	msg := make([]byte, size)

	for {
		n, err := c.conn.Read(msg)
		if err != nil {
			_ = c.forceClose()
			return
		}

		if n >= size {
			log.Warnf("the msg size too large, has been ignored")
			continue
		}

		switch c.State() {
		case network.ConnHanged:
			continue
		case network.ConnClosed:
			return
		}

		if len(msg) == 0 {
			continue
		}

		if c.connMgr.server.receiveHandler != nil {
			c.connMgr.server.receiveHandler(c, msg[:n], 0)
		}
	}
}

// 优雅关闭
func (c *serverConn) graceClose() (err error) {
	c.rw.Lock()

	if err = c.checkState(); err != nil {
		c.rw.Unlock()
		return
	}

	c.state = network.ConnHanged
	c.chWrite <- chWrite{typ: closeSig}
	c.rw.Unlock()

	<-c.done

	c.rw.Lock()
	c.state = network.ConnClosed
	close(c.chWrite)
	close(c.done)
	err = c.conn.Close()
	c.conn = nil
	c.connMgr.recycle(c)
	c.rw.Unlock()

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	return
}

// 强制关闭
func (c *serverConn) forceClose() (err error) {
	c.rw.Lock()

	if err = c.checkState(); err != nil {
		c.rw.Unlock()
		return
	}

	c.state = network.ConnClosed
	close(c.chWrite)
	close(c.done)
	err = c.conn.Close()
	c.conn = nil
	c.connMgr.recycle(c)
	c.rw.Unlock()

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	return
}

// 写入消息
func (c *serverConn) write() {
	ticker := time.NewTicker(c.connMgr.server.opts.heartbeatCheckInterval)
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

			if err := c.doWrite(write.msg); err != nil {
				log.Errorf("write message error: %v", err)
			}
		case <-ticker.C:
			deadline := time.Now().Add(-2 * c.connMgr.server.opts.heartbeatCheckInterval).Unix()
			if atomic.LoadInt64(&c.lastHeartbeatTime) < deadline {
				log.Debugf("connection heartbeat timeout")
				_ = c.Close(true)
				return
			}
		}
	}
}

// 写入消息
func (c *serverConn) doWrite(buf []byte) (err error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if c.state == network.ConnClosed {
		return
	}

	_, err = c.conn.Write(buf)

	return
}
