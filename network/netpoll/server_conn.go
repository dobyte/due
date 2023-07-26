package netpoll

import (
	"github.com/cloudwego/netpoll"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/utils/xnet"
	"github.com/dobyte/due/v2/utils/xtime"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type serverConn struct {
	rw                sync.RWMutex       // 锁
	id                int64              // 连接ID
	uid               int64              // 用户ID
	state             int32              // 连接状态
	conn              netpoll.Connection // 源连接
	connMgr           *connMgr           // 连接管理
	chWrite           chan chWrite       // 写入队列
	done              chan struct{}      // 写入完成信号
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
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	return write(c.conn.Writer(), msg)
}

// Push 发送消息（异步）
func (c *serverConn) Push(msg []byte) error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if err := c.checkState(); err != nil {
		return err
	}

	c.chWrite <- chWrite{typ: dataPacket, msg: msg}

	return nil
}

// State 获取连接状态
func (c *serverConn) State() network.ConnState {
	return network.ConnState(atomic.LoadInt32(&c.state))
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
func (c *serverConn) init(id int64, conn netpoll.Connection, cm *connMgr) error {
	c.id = id
	c.conn = conn
	c.connMgr = cm
	c.chWrite = make(chan chWrite, 1024)
	c.done = make(chan struct{})
	c.lastHeartbeatTime = xtime.Now().Unix()
	atomic.StoreInt64(&c.uid, 0)
	atomic.StoreInt32(&c.state, int32(network.ConnOpened))

	if err := c.conn.AddCloseCallback(func(connection netpoll.Connection) error {
		return c.forceClose()
	}); err != nil {
		return err
	}

	go c.write()

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}

	return nil
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
func (c *serverConn) read() error {
	if network.ConnState(atomic.LoadInt32(&c.state)) != network.ConnOpened {
		return network.ErrConnectionClosed
	}

	// block reading messages from the client
	reader := c.conn.Reader()
	defer reader.Release()

	msg, err := read(reader)
	if err != nil {
		return err
	}

	if c.connMgr.server.opts.heartbeatInterval > 0 {
		atomic.StoreInt64(&c.lastHeartbeatTime, xtime.Now().Unix())
	}

	// ignore heartbeat packet
	if len(msg) == 0 {
		return nil
	}

	if c.connMgr.server.receiveHandler != nil {
		c.connMgr.server.receiveHandler(c, msg)
	}

	return nil
}

// 优雅关闭
func (c *serverConn) graceClose() (err error) {
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
	close(c.done)
	c.conn.Close()
	c.connMgr.recycle(c)
	c.conn = nil
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
		return err
	}

	atomic.StoreInt32(&c.state, int32(network.ConnClosed))
	close(c.chWrite)
	close(c.done)
	c.conn.Close()
	c.connMgr.recycle(c)
	c.conn = nil
	c.rw.Unlock()

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	return
}

// 写入消息
func (c *serverConn) write() {
	var ticker *time.Ticker

	if c.connMgr.server.opts.heartbeatInterval > 0 {
		ticker = time.NewTicker(c.connMgr.server.opts.heartbeatInterval)
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

			err := write(c.conn.Writer(), r.msg)
			c.rw.RUnlock()

			if err != nil {
				log.Errorf("write message error: %v", err)
			}
		case <-ticker.C:
			deadline := xtime.Now().Add(-2 * c.connMgr.server.opts.heartbeatInterval).Unix()
			if atomic.LoadInt64(&c.lastHeartbeatTime) < deadline {
				log.Debugf("connection heartbeat timeout: %d", c.id)
				c.forceClose()
				return
			} else {
				c.rw.RLock()

				if atomic.LoadInt32(&c.state) == int32(network.ConnClosed) {
					c.rw.RUnlock()
					return
				}

				// send heartbeat packet
				err := write(c.conn.Writer(), nil)
				c.rw.RUnlock()

				if err != nil {
					log.Errorf("send heartbeat packet failed: %v", err)
				}
			}
		}
	}
}
