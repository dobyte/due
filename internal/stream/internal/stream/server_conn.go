package stream

import (
	"context"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/stream/internal/protocol"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/utils/xtime"
	"net"
	"sync"
	"sync/atomic"
)

type ServerConn struct {
	ctx               context.Context
	cancel            context.CancelFunc
	rw                sync.RWMutex
	conn              net.Conn // TCP源连接
	state             int32    // 连接状态
	connMgr           *connMgr // 连接管理
	lastHeartbeatTime int64    // 上次心跳时间
}

func newServerConn(cm *connMgr, conn net.Conn) *ServerConn {
	c := &ServerConn{}
	c.conn = conn
	c.connMgr = cm
	c.state = connOpened

	go c.read()

	return c
}

// Send 发送消息
func (c *ServerConn) Send(buf buffer.Buffer) (err error) {
	if err = c.checkState(); err != nil {
		return err
	}

	buf.Range(func(node *buffer.NocopyNode) bool {
		defer node.Release()
		if _, err = c.conn.Write(node.Bytes()); err != nil {
			return false
		}
		return true
	})

	return
}

// 检测连接状态
func (c *ServerConn) checkState() error {
	switch atomic.LoadInt32(&c.state) {
	case network.ConnHanged:
		return errors.ErrConnectionHanged
	case network.ConnClosed:
		return errors.ErrConnectionClosed
	default:
		return nil
	}
}

// 关闭
func (c *ServerConn) close() error {
	if !atomic.CompareAndSwapInt32(&c.state, connOpened, connClosed) {
		return errors.ErrConnectionClosed
	}

	c.cancel()

	return c.conn.Close()
}

// 读取消息
func (c *ServerConn) read() {
	conn := c.conn

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			isHeartbeat, route, _, data, err := protocol.ReadMessage(conn)
			if err != nil {
				// TODO：处理错误
				return
			}

			switch atomic.LoadInt32(&c.state) {
			case network.ConnHanged:
				continue
			case network.ConnClosed:
				return
			default:
				// ignore
			}

			atomic.StoreInt64(&c.lastHeartbeatTime, xtime.Now().UnixNano())

			if isHeartbeat {
				continue
			}

			handler, ok := c.connMgr.server.handlers[route]
			if !ok {
				continue
			}

			if err = handler(c, data); err != nil {
				// TODO：处理错误
			}
		}
	}
}
