package ws

import (
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/utils/xnet"
	"github.com/dobyte/due/v2/utils/xtime"
	"github.com/gorilla/websocket"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type clientConn struct {
	rw                sync.RWMutex    // 锁
	id                int64           // 连接ID
	uid               int64           // 用户ID
	conn              *websocket.Conn // TCP源连接
	state             int32           // 连接状态
	client            *client         // 客户端
	chWrite           chan chWrite    // 写入队列
	lastHeartbeatTime int64           // 上次心跳时间
	done              chan struct{}   // 写入完成信号
	close             chan struct{}   // 关闭信号
}

var _ network.Conn = &clientConn{}

func newClientConn(id int64, conn *websocket.Conn, client *client) network.Conn {
	c := &clientConn{
		id:                id,
		conn:              conn,
		state:             int32(network.ConnOpened),
		client:            client,
		chWrite:           make(chan chWrite, 4096),
		lastHeartbeatTime: xtime.Now().Unix(),
		done:              make(chan struct{}),
		close:             make(chan struct{}),
	}

	go c.read()

	go c.write()

	if c.client.connectHandler != nil {
		c.client.connectHandler(c)
	}

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

// Send 发送消息（异步）
// 由于gorilla/websocket库不支持一个连接得并发读写，因而使用Send方法会导致使用写锁操作
// 建议使用Push方法替代Send
func (c *clientConn) Send(msg []byte) (err error) {
	msg = packMessage(msg)

	c.rw.Lock()
	defer c.rw.Unlock()

	if err = c.checkState(); err != nil {
		return
	}

	return c.conn.WriteMessage(websocket.BinaryMessage, msg)
}

// Push 发送消息（异步）
func (c *clientConn) Push(msg []byte) (err error) {
	msg = packMessage(msg)

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

// Close 关闭连接（主动关闭）
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
	close(c.chWrite)
	close(c.close)
	close(c.done)
	c.conn.Close()
	c.conn = nil
	c.rw.Unlock()

	if c.client.disconnectHandler != nil {
		c.client.disconnectHandler(c)
	}

	return
}

// 强制关闭
func (c *clientConn) forceClose() (err error) {
	c.rw.Lock()

	if err = c.checkState(); err != nil {
		c.rw.Unlock()
		return
	}

	atomic.StoreInt32(&c.state, int32(network.ConnClosed))
	close(c.chWrite)
	close(c.close)
	close(c.done)
	c.conn.Close()
	c.conn = nil
	c.rw.Unlock()

	if c.client.disconnectHandler != nil {
		c.client.disconnectHandler(c)
	}

	return
}

// 清理连接
func (c *clientConn) cleanup() {
	c.rw.Lock()
	atomic.StoreInt32(&c.state, int32(network.ConnClosed))
	close(c.chWrite)
	close(c.done)
	c.conn = nil
	c.rw.Unlock()

	if c.client.disconnectHandler != nil {
		c.client.disconnectHandler(c)
	}
}

// 读取消息
func (c *clientConn) read() {
	for {
		select {
		case <-c.close:
			return
		default:
			msgType, msg, err := c.conn.ReadMessage()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					if _, ok := err.(*websocket.CloseError); !ok {
						log.Warnf("read message failed: %v", err)
					}
				}
				c.forceClose()
				return
			}

			if msgType != websocket.BinaryMessage {
				continue
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

			isHeartbeat, msg, err := parsePacket(msg)
			if err != nil {
				log.Errorf("parse message failed: %v", err)
				continue
			}

			// ignore heartbeat packet
			if isHeartbeat {
				continue
			}

			if c.client.receiveHandler != nil {
				c.client.receiveHandler(c, msg)
			}
		}
	}
}

// 写入消息
func (c *clientConn) write() {
	var ticker *time.Ticker

	if c.client.opts.heartbeatInterval > 0 {
		ticker = time.NewTicker(c.client.opts.heartbeatInterval)
		defer ticker.Stop()
	} else {
		ticker = &time.Ticker{C: make(chan time.Time, 1)}
	}

	for {
		select {
		case r, ok := <-c.chWrite:
			if !ok {
				return
			}

			c.rw.RLock()
			if r.typ == closeSig {
				c.done <- struct{}{}
				c.rw.RUnlock()
				return
			}

			if atomic.LoadInt32(&c.state) == int32(network.ConnClosed) {
				c.rw.RUnlock()
				return
			}

			if r.typ == heartbeatPacket {
				r.msg = packHeartbeat(false)
			}

			err := c.conn.WriteMessage(websocket.BinaryMessage, r.msg)
			c.rw.RUnlock()

			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					if _, ok := err.(*websocket.CloseError); !ok {
						log.Errorf("write message error: %v", err)
					}
				}
			}
		case <-ticker.C:
			deadline := xtime.Now().Add(-2 * c.client.opts.heartbeatInterval).Unix()
			if atomic.LoadInt64(&c.lastHeartbeatTime) < deadline {
				log.Debugf("connection heartbeat timeout: %d", c.id)
				c.forceClose()
				return
			} else {
				// send heartbeat packet
				c.rw.RLock()

				if atomic.LoadInt32(&c.state) == int32(network.ConnClosed) {
					c.rw.RUnlock()
					return
				}

				// Connections support one concurrent writer.
				c.chWrite <- chWrite{typ: heartbeatPacket}

				c.rw.RUnlock()
			}
		}
	}
}
