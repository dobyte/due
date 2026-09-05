package drpc

import (
	"bufio"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xtime"
)

type ServerConn struct {
	id                int64                              // 连接ID
	state             atomic.Int32                       // 连接状态
	connMgr           *serverConnMgr                     // 连接管理
	rw                sync.RWMutex                       // 锁
	wg1               *sync.WaitGroup                    // 读等待组
	wg2               *sync.WaitGroup                    // 写等待组
	conn              net.Conn                           // TCP源连接
	queue             *queue.Queue[*buffer.NocopyBuffer] // 高优先级队列
	lastHeartbeatTime atomic.Int64                       // 上次心跳时间
	insKind           cluster.Kind                       // 集群类型
	insID             string                             // 集群ID
}

// Send 发送消息
func (c *ServerConn) Send(buf *buffer.NocopyBuffer) (err error) {
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

// Close 关闭连接
// @param force ...bool 是否强制关闭
// @return @1 error 错误信息
func (c *ServerConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose(true)
	} else {
		return c.graceClose(true)
	}
}

// init 初始化连接
// 复用对象池中的连接对象，重置各项状态、创建读写协程并执行授权检查与连接钩子
// @param conn net.Conn TCP连接
func (c *ServerConn) init(conn net.Conn) {
	c.id = c.connMgr.id.Add(1)
	c.state.Store(int32(network.ConnOpened))
	c.conn = conn
	c.queue = queue.NewQueue[*buffer.NocopyBuffer](int32(max(128, c.connMgr.server.opts.WriteQueueSize)), c.connMgr.server.opts.WriteTimeout)
	c.lastHeartbeatTime.Store(xtime.Now().UnixNano())
	c.wg1 = &sync.WaitGroup{}
	c.wg1.Go(func() { c.read(conn) })
	c.wg2 = &sync.WaitGroup{}
	c.wg2.Go(func() { c.write(conn) })

	c.connMgr.storeConn(conn, c)
}

// reset 重置连接
// 清空连接对象内的引用与状态，以便归还对象池后安全复用
func (c *ServerConn) reset() {
	c.wg1 = nil
	c.wg2 = nil
	c.conn = nil
	c.queue = nil
	c.insID = ""
	c.insKind = 0
}

// checkState 检测连接状态
// 依据挂起/关闭状态返回对应错误，正常时返回nil
// @return @1 error 挂起返回ErrConnectionHanged，关闭返回ErrConnectionClosed，正常为nil
func (c *ServerConn) checkState() error {
	switch c.state.Load() {
	case connHanged:
		return errors.ErrConnectionHanged
	case connClosed:
		return errors.ErrConnectionClosed
	default:
		return nil
	}
}

// graceClose 优雅关闭
// 写入关闭信号等待写队列排空后关闭连接，便于尽量下发完已缓冲的消息
// @param isNeedRecycle bool 是否在关闭后将连接对象归还连接池
// @return @1 error 连接非打开态或关闭过程中出错时返回的错误
func (c *ServerConn) graceClose(isNeedRecycle bool) error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnHanged)) {
		return errors.ErrConnectionNotOpened
	}

	c.rw.RLock()
	if c.conn == nil {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}
	err := c.queue.Write(nil)
	c.rw.RUnlock()

	if err == nil {
		c.queue.Wait()
	}

	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose(isNeedRecycle)
}

// forceClose 强制关闭
// 立即切换状态为关闭并关闭连接，不等待写队列排空
// @param isNeedRecycle bool 是否在关闭后将连接对象归还连接池
// @return @1 error 连接已处于关闭态时返回的错误
func (c *ServerConn) forceClose(isNeedRecycle bool) error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose(isNeedRecycle)
}

// doClose 执行关闭操作
// 关闭写队列，等待读写协程退出后关闭TCP连接，触发断开hook，并按需归还连接对象
// @param isNeedRecycle bool 是否在关闭后将连接对象归还连接池
// @return @1 error 关闭TCP连接时的错误
func (c *ServerConn) doClose(isNeedRecycle bool) error {
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

// read 读取消息
// 持续从流中读取消息，更新心跳时间、检测空包/心跳包并分发到接收hook；读取失败时触发强制关闭
// @param conn net.Conn TCP连接
func (c *ServerConn) read(conn net.Conn) {
	var (
		header [4]byte
		reader = bufio.NewReaderSize(conn, 4096)
	)

	for {
		isHeartbeat, route, _, data, err := protocol.ReadMessage(reader, &header)
		if err != nil {
			xcall.Go(func() {
				_ = c.forceClose(true)
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

		if handler := c.connMgr.server.handlers[route]; handler != nil {
			if err := handler(c, data); err != nil && !errors.Is(err, errors.ErrNotFoundUserLocation) {
				log.Warnf("process route %d message failed: %v", route, err)
			}
		}
	}
}

// write 写入消息
// 为保证心跳能够优先下发到客户端，采用高/低优先级双队列：外层先取高优先级，空闲时在内层再取低优先级或处理心跳
// @param conn net.Conn TCP连接
func (c *ServerConn) write(conn net.Conn) {
	ticker := time.NewTicker(defaultHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case buf, ok := <-c.queue.Read():
			if !ok {
				return
			}

			if buf == nil {
				c.queue.Done(true)
				return
			}

			c.queue.Done(false)

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

// doHandleHeartbeat 处理心跳
// 检测上次收到消息的时间是否超时，超时则触发强制关闭；主动定时心跳模式下额外下发心跳包
// @param conn net.Conn TCP连接
// @param t time.Time 当前心跳触发的时间点
// @return @1 bool 是否继续写入协程循环，心跳超时时返回false
func (c *ServerConn) doHandleHeartbeat(conn net.Conn, t time.Time) bool {
	deadline := t.Add(-2 * defaultHeartbeatInterval).UnixNano()

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

// doSaveHandshakeInstance 保存握手实例
// @param insID string 集群ID
// @param insKind cluster.Kind 集群类型
func (c *ServerConn) doSaveHandshakeInstance(insID string, insKind cluster.Kind) error {
	c.rw.Lock()
	defer c.rw.Unlock()

	if err := c.checkState(); err != nil {
		return err
	}

	c.insID = insID
	c.insKind = insKind

	return nil
}
