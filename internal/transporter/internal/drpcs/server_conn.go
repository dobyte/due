package drpcs

import (
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
	wg1               *sync.WaitGroup             // 读等待组
	wg2               *sync.WaitGroup             // 写等待组
	conn              *net.TCPConn                // 连接实例
	state             atomic.Int32                // 连接状态
	queue             *queue.Queue[buffer.Buffer] // 消息队列
	dueBuffers        []buffer.Buffer             // 待写入的消息缓冲对象集合
	netBuffers        net.Buffers                 // 待写入的字节切片集合
	lastHeartbeatTime atomic.Int64                // 上次心跳时间
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
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(128, c.svr.opts.WriteQueueSize)), c.svr.opts.WriteTimeout)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.wg1 = &sync.WaitGroup{}
	c.wg1.Go(func() { c.read(conn) })

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
		isHeartbeat, rt, seq, buf, err := reader.readBuffer()
		if err != nil {
			taskpool.Add(func() { c.forceClose() })
			return
		}

		// stop read message
		if c.checkState() != nil {
			return
		}

		// ignore heartbeat packet
		if isHeartbeat {
			continue
		}

		// ignore empty packet
		if buf.Len() == 0 {
			continue
		}

		if rt == route.Handshake {
			if c.isAlived() {
				continue
			}

			if err := c.doHandshake(conn, seq, buf); err != nil {
				log.Warnf("handle handshake error: %v", err)
				taskpool.Add(func() { c.forceClose() })
				return
			}
		} else {
			if !c.isAlived() {
				continue
			}

			if err := c.svr.messageHandler(c, rt, seq, buf); err != nil {
				log.Warnf("handle message error: %v", err)
			}
		}
	}
}

// write 写入消息
// 从消息队列取出消息写入连接；同时按固定间隔检测心跳超时，超时则触发强制关闭
// @param conn net.Conn TCP连接
func (c *ServerConn) write(conn *net.TCPConn) {
	for {
		buf, ok := <-c.queue.Read()
		if !ok {
			return
		}

		c.doBatchWrite(conn, buf)
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
// @return @1 bool 是否心跳超时
func (c *ServerConn) checkHeartbeat(t *time.Time) bool {
	return c.lastHeartbeatTime.Load() >= t.Add(-2*heartbeatInterval).UnixNano()
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

// isAlived 检查连接是否存活
// @return @1 bool 是否存活
func (c *ServerConn) isAlived() bool {
	return c.state.Load() == connAlived
}

// graceClose 优雅关闭
// 写入关闭信号等待写队列排空后关闭连接，便于尽量下发完已缓冲的消息
// @return @1 error 连接非打开态或关闭过程中出错时返回的错误
func (c *ServerConn) graceClose() error {
	if !c.state.CompareAndSwap(connOpened, connHanged) {
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

	if c.state.Swap(connClosed) == connClosed {
		return errors.ErrConnectionClosed
	}

	return c.doClose()
}

// forceClose 强制关闭
// 立即切换状态为关闭并关闭连接，不等待写队列排空
// @return @1 error 连接已处于关闭态时返回的错误
func (c *ServerConn) forceClose() error {
	if c.state.Swap(connClosed) == connClosed {
		return errors.ErrConnectionClosed
	}

	return c.doClose()
}

// doClose 执行关闭操作
// 关闭写队列，等待读写协程退出后关闭TCP连接，触发断开hook，并按需归还连接对象
// @return @1 error 关闭TCP连接时的错误
func (c *ServerConn) doClose() error {
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

	for buf := range c.queue.Read() {
		buf.Release()
	}

	c.svr.deleteConn(conn)

	return err
}

// doHandshake 处理握手请求
// @param conn net.Conn TCP连接
// @param seq uint64 序列号
// @param buf *buffer.Bytes 手势请求缓冲区
// @return @1 error 处理错误
func (c *ServerConn) doHandshake(conn *net.TCPConn, seq uint64, buf *buffer.Bytes) error {
	defer buf.Release()

	kind, inst, epoch, err := protocol.DecodeHandshakeReq(buf)
	if err != nil {
		return err
	}

	if err = c.serve(kind, inst, epoch); err != nil {
		return err
	}

	res := protocol.EncodeHandshakeRes(seq, codes.OK)
	defer res.Release()

	if _, err = conn.Write(res.Bytes()); err != nil {
		return err
	}

	return nil
}

// serve 服务连接
func (c *ServerConn) serve(kind cluster.Kind, inst string, epoch uint64) error {
	c.rw.Lock()
	defer c.rw.Unlock()

	if c.conn == nil {
		return errors.ErrConnectionClosed
	}

	if !c.state.CompareAndSwap(connOpened, connAlived) {
		switch c.state.Load() {
		case connHanged:
			return errors.ErrConnectionHanged
		case connClosed:
			return errors.ErrConnectionClosed
		case connAlived:
			return errors.ErrConnectionAlived
		}
	}

	c.kind, c.inst, c.epoch = kind, inst, epoch
	c.wg2 = &sync.WaitGroup{}
	c.wg2.Go(func() { c.write(c.conn) })

	return nil
}
