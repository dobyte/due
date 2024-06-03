package server

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/utils/xtime"
	"net"
	"sync/atomic"
)

type Conn struct {
	ctx               context.Context
	cancel            context.CancelFunc
	server            *Server      // 连接管理
	conn              net.Conn     // TCP源连接
	state             int32        // 连接状态
	lastHeartbeatTime int64        // 上次心跳时间
	InsKind           cluster.Kind // 集群类型
	InsID             string       // 集群ID
}

func newConn(server *Server, conn net.Conn) *Conn {
	c := &Conn{}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.conn = conn
	c.server = server
	c.state = connOpened

	go c.read()

	go c.process()

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
	if atomic.LoadInt32(&c.state) == connClosed {
		return errors.ErrConnectionClosed
	} else {
		return nil
	}
}

// 关闭连接
func (c *Conn) close() error {
	if !atomic.CompareAndSwapInt32(&c.state, connOpened, connClosed) {
		return errors.ErrConnectionClosed
	}

	c.cancel()
	c.server.recycle(c.conn)

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
				_ = c.close()
				return
			}

			atomic.StoreInt64(&c.lastHeartbeatTime, xtime.Now().UnixNano())

			if isHeartbeat {
				continue
			}

			handler, ok := c.server.handlers[route]
			if !ok {
				continue
			}

			if err = handler(c, data); err != nil {
				// TODO：处理错误
			}
		}
	}
}

func (c *Conn) process() {

}
