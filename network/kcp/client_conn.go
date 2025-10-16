package kcp

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xnet"
	"github.com/dobyte/due/v2/utils/xtime"
	"github.com/xtaci/kcp-go/v5"
)

type clientConn struct {
	rw                sync.RWMutex
	id                int64           // 连接ID
	uid               int64           // 用户ID
	attr              *attr           // 连接属性
	conn              *kcp.UDPSession // UDP源连接
	state             atomic.Int32    // 连接状态
	client            *client         // 客户端
	chWrite           chan chWrite    // 写入队列
	done              chan struct{}   // 写入完成信号
	close             chan struct{}   // 关闭信号
	lastHeartbeatTime atomic.Int64    // 上次心跳时间
}

var _ network.Conn = &clientConn{}

func newClientConn(client *client, id int64, conn *kcp.UDPSession) network.Conn {
	c := &clientConn{
		id:      id,
		attr:    &attr{},
		conn:    conn,
		client:  client,
		chWrite: make(chan chWrite, 4096),
		done:    make(chan struct{}),
		close:   make(chan struct{}),
	}

	c.state.Store(int32(network.ConnOpened))
	c.lastHeartbeatTime.Store(xtime.Now().UnixNano())

	if c.client.opts.mtu > 0 {
		conn.SetMtu(c.client.opts.mtu)
	}

	if len(c.client.opts.noDelay) == 4 {
		conn.SetNoDelay(c.client.opts.noDelay[0], c.client.opts.noDelay[1], c.client.opts.noDelay[2], c.client.opts.noDelay[3])
	}

	if c.client.opts.ackNoDelay {
		conn.SetACKNoDelay(c.client.opts.ackNoDelay)
	}

	if c.client.opts.writeDelay {
		conn.SetWriteDelay(c.client.opts.writeDelay)
	}

	if len(c.client.opts.windowSize) == 2 {
		conn.SetWindowSize(c.client.opts.windowSize[0], c.client.opts.windowSize[1])
	}

	if c.client.opts.readBuffer > 0 {
		conn.SetReadBuffer(c.client.opts.readBuffer)
	}

	if c.client.opts.writeBuffer > 0 {
		conn.SetWriteBuffer(c.client.opts.writeBuffer)
	}

	xcall.Go(c.read)

	xcall.Go(c.write)

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

// Attr 获取属性接口
func (c *clientConn) Attr() network.Attr {
	return c.attr
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
func (c *clientConn) Send(msg []byte) error {
	if err := c.checkState(); err != nil {
		return err
	}

	c.rw.RLock()
	conn := c.conn
	c.rw.RUnlock()

	if conn == nil {
		return errors.ErrConnectionClosed
	}

	_, err := conn.Write(msg)
	return err
}

// Push 发送消息（异步）
func (c *clientConn) Push(msg []byte) error {
	if err := c.checkState(); err != nil {
		return err
	}

	c.rw.RLock()
	defer c.rw.RUnlock()

	if c.conn == nil {
		return errors.ErrConnectionClosed
	}

	c.chWrite <- chWrite{typ: dataPacket, msg: msg}

	return nil
}

// State 获取连接状态
func (c *clientConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close 关闭连接
func (c *clientConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
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
func (c *clientConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr 获取远端地址
func (c *clientConn) RemoteAddr() (net.Addr, error) {
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

// 检测连接状态
func (c *clientConn) checkState() error {
	switch c.State() {
	case network.ConnHanged:
		return errors.ErrConnectionHanged
	case network.ConnClosed:
		return errors.ErrConnectionClosed
	default:
		return nil
	}
}

// 优雅关闭
func (c *clientConn) graceClose() error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnHanged)) {
		return errors.ErrConnectionNotOpened
	}

	c.rw.RLock()
	if c.conn == nil {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}
	c.chWrite <- chWrite{typ: closeSig}
	c.rw.RUnlock()

	<-c.done

	if !c.state.CompareAndSwap(int32(network.ConnHanged), int32(network.ConnClosed)) {
		return errors.ErrConnectionNotHanged
	}

	return c.doClose()
}

// 强制关闭
func (c *clientConn) forceClose() error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnClosed)) {
		if !c.state.CompareAndSwap(int32(network.ConnHanged), int32(network.ConnClosed)) {
			return errors.ErrConnectionClosed
		}
	}

	return c.doClose()
}

// 执行关闭操作
func (c *clientConn) doClose() error {
	c.rw.Lock()

	if c.conn == nil {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	close(c.chWrite)
	close(c.close)
	close(c.done)
	conn := c.conn
	c.conn = nil
	c.rw.Unlock()

	err := conn.Close()

	if c.client.disconnectHandler != nil {
		c.client.disconnectHandler(c)
	}

	return err
}

// 读取消息
func (c *clientConn) read() {
	conn := c.conn

	for {
		select {
		case <-c.close:
			return
		default:
			msg, err := packet.ReadMessage(conn)
			if err != nil {
				_ = c.forceClose()
				return
			}

			if c.client.opts.heartbeatInterval > 0 {
				c.lastHeartbeatTime.Store(xtime.Now().UnixNano())
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

			isHeartbeat, err := packet.CheckHeartbeat(msg)
			if err != nil {
				log.Errorf("check heartbeat message error: %v", err)
				continue
			}

			// ignore heartbeat packet
			if isHeartbeat {
				continue
			}

			if c.client.receiveHandler != nil {
				c.client.receiveHandler(c, buffer.NewBytes(msg))
			}
		}
	}
}

// 写入消息
func (c *clientConn) write() {
	var (
		conn   = c.conn
		ticker *time.Ticker
	)

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

			if r.typ == closeSig {
				c.rw.RLock()
				c.done <- struct{}{}
				c.rw.RUnlock()
				return
			}

			if c.isClosed() {
				return
			}

			if _, err := conn.Write(r.msg); err != nil {
				log.Errorf("write data message error: %v", err)
			}
		case t, ok := <-ticker.C:
			if !ok {
				return
			}

			deadline := t.Add(-2 * c.client.opts.heartbeatInterval).UnixNano()

			if c.lastHeartbeatTime.Load() < deadline {
				log.Debugf("connection heartbeat timeout")
				_ = c.forceClose()
				return
			} else {
				if c.isClosed() {
					return
				}

				if heartbeat, err := packet.PackHeartbeat(); err != nil {
					log.Errorf("pack heartbeat message error: %v", err)
				} else {
					// send heartbeat packet
					if _, err := conn.Write(heartbeat); err != nil {
						log.Errorf("write heartbeat message error: %v", err)
					}
				}
			}
		}
	}
}

// 是否已关闭
func (c *clientConn) isClosed() bool {
	return network.ConnState(c.state.Load()) == network.ConnClosed
}
