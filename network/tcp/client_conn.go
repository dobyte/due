package tcp

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/internal/xnet"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/network"
)

type clientConn struct {
	rw      sync.RWMutex
	id      int64         // 连接ID
	uid     int64         // 用户ID
	conn    net.Conn      // TCP源连接
	state   int32         // 连接状态
	client  *client       // 客户端
	chWrite chan chWrite  // 写入队列
	done    chan struct{} // 写入完成信号
}

var _ network.Conn = &clientConn{}

func newClientConn(client *client, conn net.Conn) network.Conn {
	c := &clientConn{
		id:      1,
		conn:    conn,
		state:   int32(network.ConnOpened),
		client:  client,
		chWrite: make(chan chWrite, 1024),
		done:    make(chan struct{}),
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
	c.rw.RLock()
	defer c.rw.RUnlock()

	return c.uid
}

// Bind 绑定用户ID
func (c *clientConn) Bind(uid int64) {
	c.rw.Lock()
	defer c.rw.Unlock()

	c.uid = uid
}

// Send 发送消息（同步）
func (c *clientConn) Send(msg []byte, msgType ...int) error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	_, err := c.conn.Write(msg)
	return err
}

// Push 发送消息（异步）
func (c *clientConn) Push(msg []byte, msgType ...int) error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	c.chWrite <- chWrite{typ: dataPacket, msg: msg}

	return nil
}

// State 获取连接状态
func (c *clientConn) State() network.ConnState {
	return network.ConnState(atomic.LoadInt32(&c.state))
}

// Close 关闭连接
func (c *clientConn) Close(isForce ...bool) error {
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

	if c.client.disconnectHandler != nil {
		c.client.disconnectHandler(c)
	}

	return nil
}

// 关闭连接（被动关闭）
func (c *clientConn) close() {
	c.rw.Lock()
	defer c.rw.Unlock()

	if err := c.checkState(); err != nil {
		return
	}

	atomic.StoreInt32(&c.state, int32(network.ConnClosed))

	close(c.chWrite)

	if c.client.disconnectHandler != nil {
		c.client.disconnectHandler(c)
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

// 读取消息
func (c *clientConn) read() {
	for {
		msg, err := readMsgFromConn(c.conn, c.client.opts.maxMsgLength)
		if err != nil {
			if err == errMsgSizeTooLarge {
				log.Warnf("the msg size too large, has been ignored")
				continue
			}
			c.close()
			return
		}

		switch c.State() {
		case network.ConnHanged:
			continue
		case network.ConnClosed:
			return
		}

		if c.client.receiveHandler != nil {
			c.client.receiveHandler(c, msg, 0)
		}
	}
}

// 写入消息
func (c *clientConn) write() {
	ticker := time.NewTicker(c.client.opts.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.chWrite <- chWrite{typ: heartbeatPacket}
		case write, ok := <-c.chWrite:
			if !ok {
				return
			}

			if write.typ == closeSig {
				c.done <- struct{}{}
				return
			}

			buf, err := pack(write.msg)
			if err != nil {
				log.Errorf("packet message error: %v", err)
				continue
			}

			if _, err = c.conn.Write(buf); err != nil {
				log.Errorf("write message error: %v", err)
			}
		}
	}
}
