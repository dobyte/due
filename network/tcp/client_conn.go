package tcp

import (
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xnet"
	"github.com/dobyte/due/v2/utils/xtime"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type clientConn struct {
	rw                sync.RWMutex
	id                int64         // 连接ID
	uid               int64         // 用户ID
	conn              net.Conn      // TCP源连接
	state             int32         // 连接状态
	client            *client       // 客户端
	chWrite           chan chWrite  // 写入队列
	lastHeartbeatTime int64         // 上次心跳时间
	done              chan struct{} // 写入完成信号
}

var _ network.Conn = &clientConn{}

func newClientConn(client *client, id int64, conn net.Conn) network.Conn {
	c := &clientConn{
		id:                id,
		conn:              conn,
		state:             int32(network.ConnOpened),
		client:            client,
		chWrite:           make(chan chWrite, 10240),
		lastHeartbeatTime: xtime.Now().Unix(),
		done:              make(chan struct{}),
	}

	if c.client.connectHandler != nil {
		c.client.connectHandler(c)
	}

	go c.read()

	go c.write()

	return c
}

// ID 获取连接ID
func (c *clientConn) ID() int64 {
	return c.id
}

// UID 获取用户ID
func (c *clientConn) UID() int64 {
	return atomic.LoadInt64(&c.uid)
}

// Bind 绑定用户ID
func (c *clientConn) Bind(uid int64) {
	atomic.StoreInt64(&c.uid, uid)
}

// Unbind 解绑用户ID
func (c *clientConn) Unbind() {
	atomic.StoreInt64(&c.uid, 0)
}

// Send 发送消息（同步）
func (c *clientConn) Send(msg []byte) (err error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err = c.checkState(); err != nil {
		return
	}

	_, err = c.conn.Write(msg)

	return
}

// Push 发送消息（异步）
func (c *clientConn) Push(msg []byte) (err error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err = c.checkState(); err != nil {
		return
	}

	c.chWrite <- chWrite{typ: dataPacket, msg: msg}

	return
}

// State 获取连接状态
func (c *clientConn) State() network.ConnState {
	return network.ConnState(atomic.LoadInt32(&c.state))
}

// Close 关闭连接
func (c *clientConn) Close(isForce ...bool) error {
	if len(isForce) > 0 && isForce[0] {
		return c.forceClose()
	} else {
		return c.graceClose()
	}
}

// LocalIP 获取本地IP
func (c *clientConn) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr 获取本地地址
func (c *clientConn) LocalAddr() (net.Addr, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return nil, err
	}

	return c.conn.LocalAddr(), nil
}

// RemoteIP 获取远端IP
func (c *clientConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr 获取远端地址
func (c *clientConn) RemoteAddr() (net.Addr, error) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return nil, err
	}

	return c.conn.RemoteAddr(), nil
}

// 检测连接状态
func (c *clientConn) checkState() error {
	switch network.ConnState(atomic.LoadInt32(&c.state)) {
	case network.ConnHanged:
		return network.ErrConnectionHanged
	case network.ConnClosed:
		return network.ErrConnectionClosed
	}

	return nil
}

// 优雅关闭
func (c *clientConn) graceClose() (err error) {
	c.rw.Lock()

	if err = c.checkState(); err != nil {
		c.rw.Unlock()
		return
	}

	atomic.StoreInt32(&c.state, int32(network.ConnHanged))
	c.chWrite <- chWrite{typ: closeSig}
	c.rw.Unlock()

	<-c.done

	c.rw.Lock()
	atomic.StoreInt32(&c.state, int32(network.ConnClosed))
	err = c.conn.Close()
	c.rw.Unlock()

	return
}

// 强制关闭
func (c *clientConn) forceClose() error {
	c.rw.Lock()
	defer c.rw.Unlock()

	if err := c.checkState(); err != nil {
		return err
	}

	atomic.StoreInt32(&c.state, int32(network.ConnClosed))

	return c.conn.Close()
}

// 清理连接
func (c *clientConn) cleanup() {
	c.rw.Lock()
	atomic.StoreInt32(&c.state, int32(network.ConnClosed))
	close(c.chWrite)
	close(c.done)
	c.rw.Unlock()

	if c.client.disconnectHandler != nil {
		c.client.disconnectHandler(c)
	}
}

// 读取消息
func (c *clientConn) read() {
	for {
		size, msg, err := packet.Read(c.conn)
		if err != nil {
			if err != packet.ErrConnectionClosed {
				log.Warnf("read message failed: %v", err)
			}
			c.cleanup()
			return
		}

		if c.client.opts.heartbeatInterval > 0 {
			atomic.StoreInt64(&c.lastHeartbeatTime, xtime.Now().Unix())
		}

		switch c.State() {
		case network.ConnHanged:
			continue
		case network.ConnClosed:
			return
		}

		// ignore heartbeat packet
		if size == 0 {
			continue
		}

		if c.client.receiveHandler != nil {
			c.client.receiveHandler(c, msg)
		}
	}
}

// 写入消息
func (c *clientConn) write() {
	var (
		ticker    *time.Ticker
		heartbeat []byte
	)

	if c.client.opts.heartbeatInterval > 0 {
		heartbeat, _ = packet.Pack(nil)
		ticker = time.NewTicker(c.client.opts.heartbeatInterval)
		defer ticker.Stop()
	} else {
		ticker = &time.Ticker{C: make(chan time.Time, 1)}
	}

	for {
		select {
		case write, ok := <-c.chWrite:
			if !ok {
				return
			}

			c.rw.RLock()
			if write.typ == closeSig {
				c.done <- struct{}{}
				c.rw.RUnlock()
				return
			}

			if atomic.LoadInt32(&c.state) == int32(network.ConnClosed) {
				c.rw.RUnlock()
				return
			}

			if _, err := c.conn.Write(write.msg); err != nil {
				log.Errorf("write message error: %v", err)
			}

			c.rw.RUnlock()
		case <-ticker.C:
			deadline := xtime.Now().Add(-2 * c.client.opts.heartbeatInterval).Unix()
			if atomic.LoadInt64(&c.lastHeartbeatTime) < deadline {
				log.Debugf("connection heartbeat timeout")
				_ = c.forceClose()
				return
			} else {
				c.chWrite <- chWrite{typ: heartbeatPacket, msg: heartbeat}
			}
		}
	}
}
