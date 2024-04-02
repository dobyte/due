//go:build linux || freebsd || dragonfly || netbsd || openbsd || darwin
// +build linux freebsd dragonfly netbsd openbsd darwin

package gnet

import (
	"encoding/binary"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/panjf2000/gnet/v2"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/network"
	"github.com/symsimmy/due/utils/xnet"
	"github.com/symsimmy/due/utils/xtime"
	"net"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

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
func (c *serverConn) Send(msg []byte, msgType ...int) error {
	if err := c.checkState(); err != nil {
		return err
	}

	buf, err := pack(msg)
	if err != nil {
		log.Errorf("packet message error: %v", err)
		return err
	}

	err = c.conn.AsyncWrite(buf, func(conn gnet.Conn, err error) error {
		if err != nil {

		}
		return nil
	})
	return err
}

// Push 发送消息（异步）
func (c *serverConn) Push(msg []byte, msgType ...int) error {
	defer func() {
		if r := recover(); r != nil {
		}
	}()

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
	if err := c.checkState(); err != nil {
		return nil, err
	}

	return c.conn.RemoteAddr(), nil
}

// 初始化连接
func (c *serverConn) init(conn gnet.Conn, cm *serverConnMgr) error {
	c.id = cm.id
	c.conn = conn
	c.connMgr = cm
	c.chWrite = make(chan chWrite, 1024)
	c.done = make(chan struct{})
	c.close = make(chan struct{})
	c.rBlock = make(chan struct{})
	c.rRelease = make(chan struct{})
	c.wBlock = make(chan struct{})
	c.wRelease = make(chan struct{})
	c.lastHeartbeatTime = xtime.Now().Unix()
	atomic.StoreInt64(&c.uid, 0)
	atomic.StoreInt32(&c.state, int32(network.ConnOpened))

	if c.connMgr.server.connectHandler != nil {
		c.connMgr.server.connectHandler(c)
	}

	gopool.Go(c.write)

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

// 优雅关闭
func (c *serverConn) graceClose() (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("connection:[%v] uid:[%v] Recovered in f.r:%+v", c.ID(), c.UID(), r)
			c.conn.Close()
			c.connMgr.recycle(c)
			c.rw.Unlock()
		}
	}()
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
	close(c.close)
	c.conn.Close()
	c.connMgr.recycle(c)
	c.rw.Unlock()

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	return
}

// 强制关闭
func (c *serverConn) forceClose() (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("connection:[%v] uid:[%v] Recovered in f.r:%+v", c.ID(), c.UID(), r)
			c.conn.Close()
			c.connMgr.recycle(c)
			c.rw.Unlock()
		}
	}()
	c.rw.Lock()

	if err = c.checkState(); err != nil {
		c.rw.Unlock()
		return
	}

	atomic.StoreInt32(&c.state, int32(network.ConnClosed))
	close(c.chWrite)
	close(c.done)
	close(c.close)
	c.conn.Close()
	c.connMgr.recycle(c)
	c.rw.Unlock()

	if c.connMgr.server.disconnectHandler != nil {
		c.connMgr.server.disconnectHandler(c)
	}

	return
}

// 读取消息
func (c *serverConn) read(conn gnet.Conn) error {
	if network.ConnState(atomic.LoadInt32(&c.state)) != network.ConnOpened {
		return errors.New("connection is closed")
	}

	if c.connMgr.server.opts.heartbeatInterval > 0 {
		atomic.StoreInt64(&c.lastHeartbeatTime, xtime.Now().Unix())
	}

	buf, _ := conn.Peek(msgLenBytes)
	if len(buf) < msgLenBytes {
		return ErrIncompletePacket
	}

	var bodyLen uint16
	switch byteOrder() {
	case binary.LittleEndian:
		bodyLen = binary.LittleEndian.Uint16(buf[:msgLenBytes])
	case binary.BigEndian:
		bodyLen = binary.BigEndian.Uint16(buf[:msgLenBytes])
	}

	msgLen := msgLenBytes + int(bodyLen)
	if conn.InboundBuffered() < msgLen {
		return ErrIncompletePacket
	}
	if bodyLen <= 0 {
		// 收到 ping 包
		_, _ = conn.Discard(msgLenBytes)
		return ErrIncompletePacket
	}
	buf, _ = conn.Peek(msgLen)
	_, _ = conn.Discard(msgLen)

	if c.connMgr.server.receiveHandler != nil {
		c.connMgr.server.receiveHandler(c, buf[msgLenBytes:], 0)
	}

	return nil
}

// 写入消息
func (c *serverConn) write() {
	id := c.id
	uid := c.uid
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("cid:%+v,uid%+v,server_conn write task. panic: %v", id, uid, r)
		}
	}()
	var ticker *time.Ticker
	if c.connMgr.server.opts.enableHeartbeatCheck {
		ticker = time.NewTicker(c.connMgr.server.opts.heartbeatInterval)
		defer ticker.Stop()
	} else {
		ticker = &time.Ticker{C: make(chan time.Time, 1)}
	}

	writeChBacklogTicker := time.NewTicker(1 * time.Second)
	defer writeChBacklogTicker.Stop()

	for {
		select {
		case <-c.wBlock:
		inner:
			for {
				select {
				case <-c.wRelease:
					break inner
				case <-time.After(3 * time.Second):
					log.Warnf("block server write to client timeout")
					_ = c.Close(true)
					break inner
				}
			}
		case <-writeChBacklogTicker.C:
			switch c.State() {
			case network.ConnHanged:
				continue
			case network.ConnClosed:
				return
			}
			// 如果积压的数据占满了chWrite，打印log
			if len(c.chWrite) >= defaultWriteChannelSize {
				log.Warnf("cid:%+v,uid:%+v,server connection write channel backlog %+v message", c.id, c.uid, len(c.chWrite))
			}
		case write, ok := <-c.chWrite:
			if !ok {
				return
			}

			if write.typ == closeSig {
				c.done <- struct{}{}
				return
			}

			switch c.State() {
			case network.ConnHanged:
				continue
			case network.ConnClosed:
				return
			}

			buf, err := pack(write.msg)
			if err != nil {
				log.Errorf("packet message error: %v", err)
				continue
			}

			if err = c.doWrite(buf); err != nil {
				if strings.Contains(err.Error(), "connection was aborted") {
					break
				}
				//log.Errorf("connection:[%v] uid:[%v] write message error: %v", c.ID(), c.UID(), err)
			}
		case <-ticker.C:
			deadline := xtime.Now().Add(-2 * c.connMgr.server.opts.heartbeatInterval).Unix()
			if atomic.LoadInt64(&c.lastHeartbeatTime) < deadline {
				log.Infof("connection:[%v] uid:[%v] heartbeat timeout", c.ID(), c.UID())
				_ = c.Close(true)
				return
			}
		}
	}
}

// Block 阻塞conn传来的消息
func (c *serverConn) Block() {
	c.rBlock <- struct{}{}
	c.wBlock <- struct{}{}
}

// Release 释放conn传来的消息
func (c *serverConn) Release() {
	c.rRelease <- struct{}{}
	c.wRelease <- struct{}{}
}

func (c *serverConn) doWrite(buf []byte) (err error) {
	if atomic.LoadInt32(&c.state) == int32(network.ConnClosed) {
		return
	}

	// Differences between C write call and Go syscall.Write:
	// https://stackoverflow.com/questions/52081841/differences-between-c-write-call-and-go-syscall-write
	fd := c.conn.Fd()
	for len(buf) > 0 {
		n, err := syscall.Write(fd, buf)
		if err != nil {

			return err
		}
		buf = buf[n:]
	}

	return nil
}
