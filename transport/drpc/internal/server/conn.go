package server

import (
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"github.com/dobyte/due/v2/utils/xtime"
	"net"
	"sync"
	"sync/atomic"
)

type Conn struct {
	rw      sync.RWMutex
	conn    net.Conn // TCP源连接
	state   int32    // 连接状态
	connMgr *connMgr // 连接管理
	//chWrite           chan chWrite  // 写入队列
	done              chan struct{} // 写入完成信号
	close             chan struct{} // 关闭信号
	lastHeartbeatTime int64         // 上次心跳时间
}

func newConn(cm *connMgr, cn net.Conn) *Conn {
	c := &Conn{}
	c.conn = cn
	c.connMgr = cm

	go c.read()

	return c
}

// State 获取连接状态
func (c *Conn) State() network.ConnState {
	return network.ConnState(atomic.LoadInt32(&c.state))
}

func (c *Conn) Send(buf packet.IBuffer) error {
	if err := c.checkState(); err != nil {
		return err
	}

	c.rw.RLock()
	conn := c.conn
	c.rw.RUnlock()

	if conn == nil {
		return errors.ErrConnectionClosed
	}

	_, err := conn.Write(buf.Bytes())
	buf.Recycle()

	return err
}

// 检测连接状态
func (c *Conn) checkState() error {
	switch network.ConnState(atomic.LoadInt32(&c.state)) {
	case network.ConnHanged:
		return errors.ErrConnectionHanged
	case network.ConnClosed:
		return errors.ErrConnectionClosed
	default:
		return nil
	}
}

// 读取消息
func (c *Conn) read() {
	cn := c.conn

	reader := c.connMgr.server.reader

	for {
		select {
		case <-c.close:
			return
		default:
			isHeartbeat, route, _, data, err := reader.ReadMessage(cn)
			if err != nil {
				// TODO：处理错误
				return
			}

			switch c.State() {
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
