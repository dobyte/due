package server

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"
)

type chWrite struct {
	isHeartbeat bool
	buf         buffer.Buffer
}

type Conn struct {
	ctx               context.Context    // 上下文
	cancel            context.CancelFunc // 取消函数
	server            *Server            // 连接管理
	conn              net.Conn           // TCP源连接
	state             int32              // 连接状态
	chWrite           chan chWrite       // 写入通道
	lastHeartbeatTime int64              // 上次心跳时间
	InsKind           cluster.Kind       // 集群类型
	InsID             string             // 集群ID
}

func newConn(server *Server, conn net.Conn) *Conn {
	c := &Conn{}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.conn = conn
	c.server = server
	c.state = def.ConnOpened
	c.chWrite = make(chan chWrite, 4096)
	c.lastHeartbeatTime = xtime.Now().Unix()

	go c.read()

	go c.write()

	return c
}

// Send 发送消息
func (c *Conn) Send(buf buffer.Buffer) error {
	if atomic.LoadInt32(&c.state) == def.ConnClosed {
		return errors.ErrConnectionClosed
	}

	c.chWrite <- chWrite{buf: buf}

	return nil
}

// 关闭连接
func (c *Conn) close(isNeedRecycle ...bool) error {
	if !atomic.CompareAndSwapInt32(&c.state, def.ConnOpened, def.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	c.cancel()

	if len(isNeedRecycle) > 0 && isNeedRecycle[0] {
		c.server.recycle(c.conn)
	}

	err := c.conn.Close()

	time.AfterFunc(time.Second, func() {
		close(c.chWrite)
	})

	return err
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
				_ = c.close(true)
				return
			}

			if atomic.LoadInt32(&c.state) == def.ConnClosed {
				return
			}

			atomic.StoreInt64(&c.lastHeartbeatTime, xtime.Now().Unix())

			if isHeartbeat {
				c.chWrite <- chWrite{isHeartbeat: true}
			} else {
				handler, ok := c.server.handlers[route]
				if !ok {
					continue
				}

				if err := handler(c, data); err != nil && !errors.Is(err, errors.ErrNotFoundUserLocation) {
					log.Warnf("process route %d message failed: %v", route, err)
				}
			}
		}
	}
}

// 写入消息
func (c *Conn) write() {
	ticker := time.NewTicker(def.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			deadline := xtime.Now().Add(-2 * def.HeartbeatInterval).Unix()
			if atomic.LoadInt64(&c.lastHeartbeatTime) < deadline {
				_ = c.close(true)
				return
			}
		case ch, ok := <-c.chWrite:
			if !ok {
				return
			}

			if ch.isHeartbeat {
				if _, err := c.conn.Write(protocol.Heartbeat()); err != nil {
					log.Warnf("write heartbeat message error: %v", err)
				}
			} else {
				ok = ch.buf.Visit(func(node *buffer.NocopyNode) bool {
					if _, err := c.conn.Write(node.Bytes()); err != nil {
						log.Warnf("write buffer message error: %v", err)
						return false
					}
					return true
				})

				ch.buf.Release()

				if !ok {
					return
				}
			}
		}
	}
}
