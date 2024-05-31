package server

import (
	"context"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/utils/xtime"
	"net"
	"sync"
	"sync/atomic"
)

type Conn struct {
	ctx               context.Context
	cancel            context.CancelFunc
	rw                sync.RWMutex
	conn              net.Conn // TCP源连接
	state             int32    // 连接状态
	connMgr           *ConnMgr // 连接管理
	lastHeartbeatTime int64    // 上次心跳时间
}

func newConn(cm *ConnMgr, conn net.Conn) *Conn {
	c := &Conn{}
	c.conn = conn
	c.connMgr = cm
	c.state = connOpened

	go c.read()

	return c
}

// Send 发送消息
func (c *Conn) Send(buf buffer.Buffer) (err error) {
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
func (c *Conn) checkState() error {
	//switch atomic.LoadInt32(&c.state) {
	//case network.ConnHanged:
	//	return errors.ErrConnectionHanged
	//case network.ConnClosed:
	//	return errors.ErrConnectionClosed
	//default:
	//	return nil
	//}

	return nil
}

// 关闭
func (c *Conn) close() error {
	if !atomic.CompareAndSwapInt32(&c.state, connOpened, connClosed) {
		return errors.ErrConnectionClosed
	}

	c.cancel()

	return c.conn.Close()
}

// 读取消息
func (c *Conn) read() {
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

			//switch atomic.LoadInt32(&c.state) {
			//case network.ConnHanged:
			//	continue
			//case network.ConnClosed:
			//	return
			//default:
			//	// ignore
			//}

			atomic.StoreInt64(&c.lastHeartbeatTime, xtime.Now().UnixNano())

			if isHeartbeat {
				continue
			}

			handler, ok := c.connMgr.server.handlers[uint8(route)]
			if !ok {
				continue
			}

			if err = handler(c, data); err != nil {
				// TODO：处理错误
			}
		}
	}
}
