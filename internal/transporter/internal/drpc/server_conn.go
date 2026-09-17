package drpc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/log"
	taskpool "github.com/dobyte/due/v2/task"
)

type ServerConn struct {
	svr               *Server                     // 服务器
	rw                sync.RWMutex                // 锁
	ctx               context.Context             // 上下文
	cancel            context.CancelFunc          // 取消函数
	wg1               *sync.WaitGroup             // 读等待组
	wg2               *sync.WaitGroup             // 写等待组
	conn              *net.TCPConn                // 连接实例
	state             atomic.Int32                // 连接状态
	queue             *queue.Queue[buffer.Buffer] // 消息队列
	dueBuffers        []buffer.Buffer             // 待写入的消息缓冲对象集合
	netBuffers        net.Buffers                 // 待写入的字节切片集合
	lastHeartbeatTime atomic.Int64                // 上次心跳时间
	key               string                      // 连接键值
	kind              cluster.Kind                // 实例类型
	inst              string                      // 实例ID
	epoch             uint64                      // 连接时间戳
}

func newServerConn(svr *Server, conn *net.TCPConn) *ServerConn {
	c := &ServerConn{}
	c.svr = svr
	c.conn = conn
	c.conn.SetNoDelay(true)
	c.state.Store(connOpened)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(128, c.svr.opts.WriteQueueSize)), c.svr.opts.WriteTimeout)
	c.wg1 = &sync.WaitGroup{}
	c.wg1.Go(func() { c.read(conn) })
	c.wg2 = &sync.WaitGroup{}
	c.wg2.Go(func() { c.write(c.conn) })

	return c
}

// Push 推送消息
func (c *ServerConn) Push(buf *buffer.NocopyBuffer) error {
	c.rw.RLock()

	if err := c.checkState(); err != nil {
		c.rw.RUnlock()

		buf.Release()

		return err
	}

	if err := c.queue.Write(buf); err != nil {
		c.rw.RUnlock()

		buf.Release()

		return err
	}

	c.rw.RUnlock()

	return nil
}

// read 读取消息
// 持续从流中读取消息，更新心跳时间、检测空包/心跳包并分发到接收hook；读取失败时触发强制关闭
// @param conn net.Conn TCP连接
func (c *ServerConn) read(conn *net.TCPConn) {
	reader := newReader(conn)

	for {
		isHeartbeat, rt, seq, buf, err := reader.read()
		if err != nil {
			taskpool.Add(func() { c.forceClose() })
			return
		}

		switch state := c.state.Load(); state {
		case connClosed:
			if !isHeartbeat {
				buf.Release()
			}
			return
		case connHanged:
			if isHeartbeat {
				c.lastHeartbeatTime.Store(time.Now().UnixNano())
			} else {
				buf.Release()
				return
			}
		default:
			// ignore heartbeat packet
			if isHeartbeat {
				c.lastHeartbeatTime.Store(time.Now().UnixNano())
			} else {
				// ignore empty packet
				if buf.Len() == 0 {
					buf.Release()
					continue
				}

				if rt == route.Handshake {
					if state == connAlived {
						buf.Release()
						continue
					}

					if err := c.doHandshake(seq, buf); err != nil {
						log.Warnf("handle handshake error: %v", err)
						taskpool.Add(func() { c.forceClose() })
						return
					}
				} else {
					if state != connAlived {
						buf.Release()
						continue
					}

					if err := c.svr.messageHandler(c, rt, seq, buf); err != nil {
						log.Warnf("handle message error: %v", err)
					}
				}
			}
		}
	}
}

// write 写入消息
// 从消息队列取出消息写入连接；同时按固定间隔检测心跳超时，超时则触发强制关闭
// @param conn net.Conn TCP连接
func (c *ServerConn) write(conn *net.TCPConn) {
	for {
		if c.ctx.Err() != nil {
			return
		}

		select {
		case <-c.ctx.Done():
			return
		case buf, ok := <-c.queue.Read():
			if !ok {
				return
			}

			c.doBatchWrite(conn, buf)
		}
	}
}

// doBatchWrite 批量写入消息
// 从写队列批量取出任务，收集字节后通过net.Buffers一次性下发，减少系统调用次数
// @param conn net.Conn TCP连接
// @param first buffer.Buffer 首个已取出的任务
func (c *ServerConn) doBatchWrite(conn net.Conn, first buffer.Buffer) {
	closeSig := first.Len() == 0

	c.queue.Done(closeSig)

	if closeSig {
		first.Release()
		return
	}

	c.dueBuffers = c.dueBuffers[:0]
	c.dueBuffers = append(c.dueBuffers, first)

	for len(c.dueBuffers) < maxBatchWriteNum {
		select {
		case <-c.ctx.Done():
			goto OVER
		case buf, ok := <-c.queue.Read():
			if !ok {
				goto OVER
			}

			closeSig = buf.Len() == 0

			c.queue.Done(closeSig)

			if closeSig {
				buf.Release()
				goto OVER
			}

			c.dueBuffers = append(c.dueBuffers, buf)
		default:
			goto OVER
		}
	}

OVER:
	c.netBuffers = c.netBuffers[:0]

	for _, buf := range c.dueBuffers {
		buf.VisitBytes(func(bytes []byte) bool {
			c.netBuffers = append(c.netBuffers, bytes)
			return true
		})
	}

	if len(c.netBuffers) > 0 {
		if c.svr.opts.WriteTimeout > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(c.svr.opts.WriteTimeout))
		}

		if _, err := c.netBuffers.WriteTo(conn); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Errorf("write message error: %v", err)
				taskpool.Add(func() { c.forceClose() })
			}
		}
	}

	for _, buf := range c.dueBuffers {
		buf.Release()
	}

	c.netBuffers = c.netBuffers[:0]
	c.dueBuffers = c.dueBuffers[:0]
}

// checkHeartbeat 检查心跳是否超时
// @param t *time.Time 当前心跳触发的时间点
func (c *ServerConn) checkHeartbeat(t *time.Time) {
	if c.lastHeartbeatTime.Load() < t.Add(-2*heartbeatInterval).UnixNano() {
		taskpool.Add(func() { c.forceClose() })
	}
}

// checkState 检测连接状态
// 依据挂起/关闭状态返回对应错误，正常时返回nil
// @return @1 error 挂起返回ErrConnectionHanged，关闭返回ErrConnectionClosed，正常为nil
func (c *ServerConn) checkState() error {
	switch c.state.Load() {
	case connOpened:
		return errors.ErrConnectionNotAlived
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
// @return @1 error 连接非打开态或关闭过程中出错时返回的错误
func (c *ServerConn) graceClose() error {
	c.rw.RLock()
	switch {
	case c.state.CompareAndSwap(connOpened, connHanged):
		// ignore
	case c.state.CompareAndSwap(connAlived, connHanged):
		// ignore
	default:
		c.rw.RUnlock()
		return errors.ErrConnectionNotOpened
	}

	if c.conn == nil {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}

	err := c.queue.Write(buffer.NewBytes(nil))

	c.rw.RUnlock()

	if err == nil {
		c.queue.Wait()
	}

	return c.forceClose()
}

// forceClose 强制关闭
// 立即切换状态为关闭并关闭连接，不等待写队列排空
// @return @1 error 连接已处于关闭态时返回的错误
func (c *ServerConn) forceClose() error {
	c.rw.Lock()

	if c.state.Swap(connClosed) == connClosed {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	if c.conn == nil {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	c.cancel()
	conn := c.conn
	c.conn = nil
	key := c.key
	c.rw.Unlock()

	// 队列尚未关闭，写协程只能通过ctx退出，不再排空残留消息
	c.wg2.Wait()

	// 关闭队列，使回放时的range能读完残留消息后正常终止
	c.queue.Close()

	if key != "" {
		c.svr.doCacheQueue(key, c.queue)
	}

	err := conn.Close()

	c.wg1.Wait()
	c.svr.deleteConn(conn)

	return err
}

// doHandshake 处理握手请求
// @param seq uint64 序列号
// @param buf buffer.Buffer 手势请求缓冲区
// @return @1 error 处理错误
func (c *ServerConn) doHandshake(seq uint64, buf buffer.Buffer) error {
	kind, inst, epoch, err := protocol.DecodeHandshakeReq(buf)
	buf.Release()
	if err != nil {
		return err
	}

	buf = protocol.EncodeHandshakeRes(seq, codes.OK)

	c.rw.Lock()
	defer c.rw.Unlock()

	if c.conn == nil {
		buf.Release()
		return errors.ErrConnectionClosed
	}

	if !c.state.CompareAndSwap(connOpened, connAlived) {
		switch c.state.Load() {
		case connHanged:
			buf.Release()
			return errors.ErrConnectionHanged
		case connClosed:
			buf.Release()
			return errors.ErrConnectionClosed
		case connAlived:
			buf.Release()
			return errors.ErrConnectionAlived
		}
	}

	if err := c.queue.Write(buf); err != nil {
		buf.Release()
		return err
	}

	c.key = fmt.Sprintf("%s:%s:%d", kind, inst, epoch)
	c.kind, c.inst, c.epoch = kind, inst, epoch

	if queue, ok := c.svr.doLoadQueue(c.key); ok {
		for buf = range queue.Read() {
			if err := c.queue.Write(buf); err != nil {
				buf.Release()
				log.Warnf("write cache message failed: %v", err)
			}
		}
	}

	return nil
}
