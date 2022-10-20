package ws

import (
	"github.com/dobyte/due/internal/xnet"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/network"
	"github.com/gorilla/websocket"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type clientConn struct {
	rw      sync.RWMutex    // 锁
	id      int64           // 连接ID
	uid     int64           // 用户ID
	conn    *websocket.Conn // TCP源连接
	state   int32           // 连接状态
	client  *client         // 客户端
	chWrite chan chWrite    // 写入队列
	done    chan struct{}   // 写入完成信号
}

var _ network.Conn = &clientConn{}

func newClientConn(client *client, conn *websocket.Conn) network.Conn {
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
	if err := c.checkState(); err != nil {
		return err
	}

	if len(msgType) == 0 {
		msgType = append(msgType, TextMessage)
	}

	switch msgType[0] {
	case TextMessage, BinaryMessage:
		c.rw.RLock()
		defer c.rw.RUnlock()

		return c.conn.WriteMessage(msgType[0], msg)
	default:
		return network.ErrIllegalMsgType
	}
}

// Push 发送消息（异步）
func (c *clientConn) Push(msg []byte, msgType ...int) error {
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
func (c *clientConn) State() network.ConnState {
	return network.ConnState(atomic.LoadInt32(&c.state))
}

// Close 关闭连接（主动关闭）
func (c *clientConn) Close(isForce ...bool) (err error) {
	if len(isForce) == 0 || !isForce[0] {
		if err = c.changeState(network.ConnHanged); err != nil {
			return
		}
		c.chWrite <- chWrite{typ: closeSig}
		<-c.done
		atomic.StoreInt32(&c.state, int32(network.ConnClosed))
	} else {
		if err = c.changeState(network.ConnClosed); err != nil {
			return
		}
	}

	c.rw.Lock()
	close(c.chWrite)
	close(c.done)
	err = c.conn.Close()
	c.rw.Unlock()

	if c.client.disconnectHandler != nil {
		c.client.disconnectHandler(c)
	}

	return
}

// 关闭连接（被动关闭）
func (c *clientConn) close() {
	c.rw.Lock()
	if err := c.checkState(); err != nil {
		c.rw.Unlock()
		return
	}
	atomic.StoreInt32(&c.state, int32(network.ConnClosed))
	close(c.chWrite)
	close(c.done)
	c.rw.Unlock()

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

// 变更连接状态
func (c *clientConn) changeState(state network.ConnState) (err error) {
	c.rw.Lock()
	defer c.rw.RLock()

	if err = c.checkState(); err == nil {
		atomic.StoreInt32(&c.state, int32(state))
	}

	return
}

// 读取消息
func (c *clientConn) read() {
	for {
		msgType, buf, err := c.conn.ReadMessage()
		if err != nil {
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
			c.client.receiveHandler(c, buf, msgType)
		}
	}
}

// 写入消息
func (c *clientConn) write() {
	var ticker *time.Ticker
	if c.client.opts.enableHeartbeat {
		ticker = time.NewTicker(c.client.opts.heartbeatInterval)
		defer ticker.Stop()
	} else {
		ticker = &time.Ticker{C: make(chan time.Time, 1)}
	}

	for {
		select {
		case <-ticker.C:
			c.chWrite <- chWrite{typ: heartbeatPacket, msgType: BinaryMessage}
		case write, ok := <-c.chWrite:
			if !ok {
				return
			}

			if c.State() == network.ConnClosed {
				return
			}

			if write.typ == closeSig {
				c.done <- struct{}{}
				return
			}

			c.rw.RLock()
			err := c.conn.WriteMessage(write.msgType, write.msg)
			c.rw.RUnlock()
			if err != nil {
				log.Errorf("write message error: %v", err)
			}
		}
	}
}
