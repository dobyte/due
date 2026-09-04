package server

import (
	"bufio"
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xtime"
)

type Conn struct {
	ctx     context.Context    // 上下文
	cancel  context.CancelFunc // 取消函数
	server  *Server            // 连接管理
	InsKind cluster.Kind       // 集群类型
	InsID   string             // 集群ID

	id                int64                              // 连接ID
	conn              net.Conn                           // TCP源连接
	rw                sync.RWMutex                       // 锁
	state             atomic.Int32                       // 连接状态
	wg1               *sync.WaitGroup                    // 读等待组
	wg2               *sync.WaitGroup                    // 写等待组
	queue             *queue.Queue[*buffer.NocopyBuffer] // 消息队列
	lastHeartbeatTime atomic.Int64                       // 上次心跳时间
}

func newConn(server *Server, conn net.Conn) *Conn {
	c := &Conn{}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.conn = conn
	c.server = server
	c.state.Store(def.ConnOpened)
	c.queue = queue.NewQueue[*buffer.NocopyBuffer](int32(max(128, c.server.opts.WriteQueueSize)), c.server.opts.WriteTimeout)
	c.lastHeartbeatTime.Store(xtime.Now().Unix())
	c.wg1 = &sync.WaitGroup{}
	c.wg1.Go(func() { c.read(conn) })
	c.wg2 = &sync.WaitGroup{}
	c.wg2.Go(func() { c.write(conn) })

	return c
}

// Send 发送消息
func (c *Conn) Send(buf *buffer.NocopyBuffer) (err error) {
	defer func() {
		if err != nil {
			buf.Release()
		}
	}()

	c.rw.RLock()
	defer c.rw.RUnlock()

	if err = c.checkState(); err != nil {
		return
	}

	if err = c.queue.Write(buf); err != nil {
		return
	}

	return
}

// checkState 检测连接状态
// 依据挂起/关闭状态返回对应错误，正常时返回nil
// @return @1 error 挂起返回ErrConnectionHanged，关闭返回ErrConnectionClosed，正常为nil
func (c *Conn) checkState() error {
	switch c.state.Load() {
	case def.ConnHanged:
		return errors.ErrConnectionHanged
	case def.ConnClosed:
		return errors.ErrConnectionClosed
	default:
		return nil
	}
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

// read 读取消息
// 持续从流中读取消息，更新心跳时间、检测空包/心跳包并分发到接收hook；读取失败时触发强制关闭
// @param conn net.Conn TCP连接
func (c *Conn) read(conn net.Conn) {
	var (
		header [4]byte
		reader = bufio.NewReaderSize(conn, 4096)
	)

	for {
		isHeartbeat, route, _, data, err := protocol.ReadMessage(reader, &header)
		if err != nil {
			xcall.Go(func() {
				_ = c.close(true)
			})
			return
		}

		c.lastHeartbeatTime.Store(xtime.Now().UnixNano())

		// stop read message
		if c.checkState() != nil {
			return
		}

		// ignore empty packet
		if len(data) == 0 {
			continue
		}

		// ignore heartbeat packet
		if isHeartbeat {
			continue
		}

		if handler := c.server.handlers[route]; handler != nil {
			if err := handler(c, data); err != nil && !errors.Is(err, errors.ErrNotFoundUserLocation) {
				log.Warnf("process route %d message failed: %v", route, err)
			}
		}
	}
}

// write 写入消息
// 为保证心跳能够优先下发到客户端，采用高/低优先级双队列：外层先取高优先级，空闲时在内层再取低优先级或处理心跳
// @param conn net.Conn TCP连接
func (c *Conn) write(conn net.Conn) {
	ticker := time.NewTicker(def.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case buf, ok := <-c.queue.Read():
			if !ok {
				return
			}

			c.queue.Done(buf == nil)

			ok = buf.Visit(func(node *buffer.NocopyNode) bool {
				if _, err := c.conn.Write(node.Bytes()); err != nil {
					log.Warnf("write buffer message error: %v", err)
					return false
				}
				return true
			})

			buf.Release()

			if !ok {
				return
			}
		case t, ok := <-ticker.C:
			if !ok {
				return
			}

			if !c.doHandleHeartbeat(conn, t) {
				return
			}
		}
	}
}

// forceClose 强制关闭
// 立即切换状态为关闭并关闭连接，不等待写队列排空
// @param isNeedRecycle bool 是否在关闭后将连接对象归还连接池
// @return @1 error 连接已处于关闭态时返回的错误
func (c *Conn) forceClose(isNeedRecycle bool) error {
	if c.state.Swap(def.ConnClosed) == def.ConnClosed {
		return errors.ErrConnectionClosed
	}

	return c.doClose(isNeedRecycle)
}

// doClose 执行关闭操作
// 关闭写队列，等待读写协程退出后关闭TCP连接，触发断开hook，并按需归还连接对象
// @param isNeedRecycle bool 是否在关闭后将连接对象归还连接池
// @return @1 error 关闭TCP连接时的错误
func (c *Conn) doClose(isNeedRecycle bool) error {
	c.rw.Lock()
	if c.conn == nil {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	c.queue.Close()
	conn := c.conn
	c.conn = nil
	c.rw.Unlock()

	c.wg2.Wait()

	err := conn.Close()

	c.wg1.Wait()

	if isNeedRecycle {
		c.connMgr.recycleConn(conn)
	}

	return err
}

// doHandleHeartbeat 处理心跳
// 检测上次收到消息的时间是否超时，超时则触发强制关闭；主动定时心跳模式下额外下发心跳包
// @param conn net.Conn TCP连接
// @param t time.Time 当前心跳触发的时间点
// @return @1 bool 是否继续写入协程循环，心跳超时时返回false
func (c *Conn) doHandleHeartbeat(conn net.Conn, t time.Time) bool {
	deadline := t.Add(-2 * c.connMgr.server.opts.heartbeatInterval).UnixNano()

	if c.lastHeartbeatTime.Load() < deadline {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		xcall.Go(func() {
			_ = c.forceClose(true)
		})

		return false
	} else {
		return true
	}
}
