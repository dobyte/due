package quic

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	taskpool "github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/utils/xnet"
	"github.com/quic-go/quic-go"
)

type clientConn struct {
	rw                sync.RWMutex                // 锁
	id                int64                       // 连接ID
	uid               atomic.Int64                // 用户ID
	attr              *attr                       // 连接属性
	qc                *quic.Conn                  // QUIC连接
	stream            *quic.Stream                // 双向流
	state             atomic.Int32                // 连接状态
	cli               *client                     // 客户端
	wg1               *sync.WaitGroup             // 读等待组
	wg2               *sync.WaitGroup             // 写等待组
	queue             *queue.Queue[buffer.Buffer] // 消息队列
	output            *bufferWriter               // 写辅助对象
	dueBuffers        []buffer.Buffer             // 待写入的消息缓冲对象集合
	lastHeartbeatTime atomic.Int64                // 上次心跳时间
}

var _ network.Conn = &clientConn{}

// newClientConn 创建客户端连接
// @param cli *client 客户端
// @param qc *quic.Conn QUIC连接
// @param stream *quic.Stream 双向流
// @return @1 network.Conn 连接对象
func newClientConn(cli *client, qc *quic.Conn, stream *quic.Stream) network.Conn {
	c := &clientConn{}
	c.id = cli.cid.Add(1)
	c.attr = &attr{}
	c.qc = qc
	c.stream = stream
	c.cli = cli
	c.state.Store(int32(network.ConnOpened))
	c.queue = queue.NewQueue[buffer.Buffer](int32(max(minWriteQueueSize, cli.opts.writeQueueSize)), cli.opts.writeTimeout)
	c.output = newBufferWriter(stream)
	c.dueBuffers = make([]buffer.Buffer, 0, maxBatchWriteNum)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	c.wg1 = &sync.WaitGroup{}
	c.wg2 = &sync.WaitGroup{}

	c.wg2.Go(func() { c.write(stream) })
	c.wg1.Go(func() {
		// OnConnect 在读循环之前执行，保证其始终先于 OnReceive 触发
		if cli.connectHandler != nil {
			cli.connectHandler(c)
		}
		c.read(stream)
	})

	return c
}

// ID 获取连接ID
// @return @1 int64 连接ID
func (c *clientConn) ID() int64 {
	return c.id
}

// UID 获取用户ID
// @return @1 int64 用户ID，未绑定时为0
func (c *clientConn) UID() int64 {
	return c.uid.Load()
}

// Attr 获取属性接口
// @return @1 network.Attr 属性接口
func (c *clientConn) Attr() network.Attr {
	return c.attr
}

// Bind 绑定用户ID
// @param uid int64 用户ID
// @return @1 error 错误信息
func (c *clientConn) Bind(uid int64) error {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}

	c.uid.Store(uid)

	c.rw.RUnlock()

	return nil
}

// Unbind 解绑用户ID
// @return @1 error 错误信息
func (c *clientConn) Unbind() error {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}

	c.uid.Store(0)

	c.rw.RUnlock()

	return nil
}

// Push 发送消息
// 将消息写入发送队列；仅当返回nil时 buf 的所有权才转移给网络层
// @param buf buffer.Buffer 消息内容，消息发送失败自行控制释放buffer
// @return @1 error 错误信息
func (c *clientConn) Push(buf buffer.Buffer) error {
	if buf == nil || buf.Len() == 0 {
		return errors.ErrInvalidMessage
	}

	c.rw.RLock()

	if err := c.checkState(); err != nil {
		c.rw.RUnlock()
		return err
	}

	err := c.queue.Write(buf)

	c.rw.RUnlock()

	return err
}

// State 获取连接状态
// @return @1 network.ConnState 连接状态
func (c *clientConn) State() network.ConnState {
	return network.ConnState(c.state.Load())
}

// Close 关闭连接
// @param force ...bool 是否强制关闭
// @return @1 error 错误信息
func (c *clientConn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose()
	}
	return c.graceClose()
}

// LocalIP 获取本地IP
// @return @1 string 本地IP地址
// @return @2 error 错误信息
func (c *clientConn) LocalIP() (string, error) {
	addr, err := c.LocalAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// LocalAddr 获取本地地址
// @return @1 net.Addr 本地地址
// @return @2 error 错误信息
func (c *clientConn) LocalAddr() (net.Addr, error) {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return nil, errors.ErrConnectionClosed
	}

	qc := c.qc
	c.rw.RUnlock()

	return qc.LocalAddr(), nil
}

// RemoteIP 获取远端IP
// @return @1 string 远端IP地址
// @return @2 error 错误信息
func (c *clientConn) RemoteIP() (string, error) {
	addr, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}

	return xnet.ExtractIP(addr)
}

// RemoteAddr 获取远端地址
// @return @1 net.Addr 远端地址
// @return @2 error 错误信息
func (c *clientConn) RemoteAddr() (net.Addr, error) {
	c.rw.RLock()

	if c.isClosed() {
		c.rw.RUnlock()
		return nil, errors.ErrConnectionClosed
	}

	qc := c.qc
	c.rw.RUnlock()

	return qc.RemoteAddr(), nil
}

// checkState 检测连接状态
// 依据挂起/关闭状态返回对应错误，正常时返回nil
// @return @1 error 挂起返回ErrConnectionHanged，关闭返回ErrConnectionClosed，正常为nil
func (c *clientConn) checkState() error {
	switch c.State() {
	case network.ConnHanged:
		return errors.ErrConnectionHanged
	case network.ConnClosed:
		return errors.ErrConnectionClosed
	default:
		return nil
	}
}

// graceClose 优雅关闭
// 写入关闭信号等待写队列排空后关闭连接，便于尽量下发完已缓冲的消息；
// 配置优雅关闭超时时间后，超时未排空将直接断开底层连接以强制结束等待
// @return @1 error 连接非打开态或关闭过程中出错时返回的错误
func (c *clientConn) graceClose() error {
	if !c.state.CompareAndSwap(int32(network.ConnOpened), int32(network.ConnHanged)) {
		return errors.ErrConnectionNotOpened
	}

	c.rw.RLock()
	if c.qc == nil {
		c.rw.RUnlock()
		return errors.ErrConnectionClosed
	}
	qc := c.qc
	q := c.queue
	err := q.Write(buffer.NewBytes(nil))
	c.rw.RUnlock()

	if err == nil {
		if closeTimeout := c.cli.opts.closeTimeout; closeTimeout > 0 {
			// 排空超时后强制断开底层连接，打断写协程中可能阻塞的写操作
			timer := time.AfterFunc(closeTimeout, func() { _ = qc.CloseWithError(0, "close timeout") })
			q.Wait()
			timer.Stop()
		} else {
			q.Wait()
		}
	}

	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose(true)
}

// forceClose 强制关闭
// 立即切换状态为关闭并关闭连接，不等待写队列排空
// @return @1 error 连接已处于关闭态时返回的错误
func (c *clientConn) forceClose() error {
	if c.state.Swap(int32(network.ConnClosed)) == int32(network.ConnClosed) {
		return errors.ErrConnectionClosed
	}

	return c.doClose(false)
}

// doClose 执行关闭操作
// 关闭写队列，等待读写协程退出后关闭流与QUIC连接，并触发断开hook；
// graceful 为 true 时先等待写协程排空并发送FIN、驻留closeTimeout等待对端确认
// @param graceful bool 是否优雅关闭
// @return @1 error 关闭QUIC连接时的错误
func (c *clientConn) doClose(graceful bool) error {
	c.rw.Lock()
	if c.qc == nil {
		c.rw.Unlock()
		return errors.ErrConnectionClosed
	}

	c.queue.Close()
	qc := c.qc
	stream := c.stream
	c.qc = nil
	c.stream = nil
	c.rw.Unlock()

	if graceful {
		// 等待写协程排空所有已接收的消息后再发送FIN
		c.wg2.Wait()

		_ = stream.Close()

		// 在 closeTimeout 内保持传输层存活，使对端能够确认FIN并重传，随后再将其拆除
		select {
		case <-qc.Context().Done():
		case <-time.After(c.cli.opts.closeTimeout):
		}
	}

	err := qc.CloseWithError(0, "closed")

	c.wg2.Wait()
	c.wg1.Wait()

	for buf := range c.queue.Read() {
		buf.Release()
	}

	if c.cli.disconnectHandler != nil {
		c.cli.disconnectHandler(c)
	}

	return err
}

// read 读取消息
// 持续从流中读取消息，更新心跳时间、检测空包/心跳包并分发到接收hook；读取失败时触发强制关闭
// @param stream *quic.Stream 双向流
func (c *clientConn) read(stream *quic.Stream) {
	var index = 0

	for {
		isHeartbeat, heartbeatTime, buf, err := packet.Read(stream)
		if err != nil {
			taskpool.Add(func() { c.forceClose() })
			return
		}

		switch c.State() {
		case network.ConnClosed:
			if !isHeartbeat {
				buf.Release()
			}
			return
		case network.ConnHanged:
			if !isHeartbeat {
				buf.Release()
				return
			}
		}

		if isHeartbeat {
			if c.cli.opts.heartbeatInterval > 0 {
				c.lastHeartbeatTime.Store(time.Now().UnixNano())
			}

			if c.cli.heartbeatHandler != nil {
				c.cli.heartbeatHandler(c, heartbeatTime)
			}
		} else {
			if c.cli.opts.heartbeatInterval > 0 {
				index++
				if index%10 == 0 {
					c.lastHeartbeatTime.Store(time.Now().UnixNano())
				}
			}

			if buf.Len() == 0 {
				buf.Release()
				continue
			}

			if c.cli.receiveHandler != nil {
				c.cli.receiveHandler(c, buf)
			} else {
				buf.Release()
			}
		}
	}
}

// write 写入消息
// 从写队列批量取出消息写入流，并按心跳间隔触发心跳检测与下发
// @param stream *quic.Stream 双向流
func (c *clientConn) write(stream *quic.Stream) {
	var tickerC <-chan time.Time

	if c.cli.opts.heartbeatInterval > 0 {
		ticker := time.NewTicker(c.cli.opts.heartbeatInterval)
		defer ticker.Stop()
		tickerC = ticker.C
	}

	for {
		select {
		case buf, ok := <-c.queue.Read():
			if !ok {
				return
			}

			c.doBatchWrite(stream, buf)
		case t, ok := <-tickerC:
			if !ok {
				return
			}

			if !c.doHandleHeartbeat(stream, t) {
				return
			}
		}
	}
}

// doBatchWrite 批量写入消息
// 从写队列批量取出任务后逐条写入，减少队列通道操作次数；
// QUIC流写入为用户态拷贝，无需像TCP那样聚合字节切片一次性下发
// @param stream *quic.Stream 双向流
// @param first buffer.Buffer 首个已取出的任务
func (c *clientConn) doBatchWrite(stream *quic.Stream, first buffer.Buffer) {
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
	for _, buf := range c.dueBuffers {
		if c.cli.opts.writeTimeout > 0 {
			_ = stream.SetWriteDeadline(time.Now().Add(c.cli.opts.writeTimeout))
		}

		if err := c.output.write(buf); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Warnf("write message error: %v", err)
			taskpool.Add(func() { c.forceClose() })
		}

		buf.Release()
	}

	c.dueBuffers = c.dueBuffers[:0]
}

// doHandleHeartbeat 处理心跳
// 检测上次收到消息的时间是否超时，超时则触发强制关闭，未超时则下发心跳包
// @param stream *quic.Stream 双向流
// @param t time.Time 当前心跳触发的时间点
// @return @1 bool 是否继续写入协程循环，心跳超时时返回false
func (c *clientConn) doHandleHeartbeat(stream *quic.Stream, t time.Time) bool {
	if c.lastHeartbeatTime.Load() < t.Add(-2*c.cli.opts.heartbeatInterval).UnixNano() {
		log.Debugf("connection heartbeat timeout, cid: %d", c.id)

		taskpool.Add(func() { c.forceClose() })

		return false
	}

	if c.cli.opts.writeTimeout > 0 {
		_ = stream.SetWriteDeadline(time.Now().Add(c.cli.opts.writeTimeout))
	}

	hb := packet.PackHeartbeat()

	if err := c.output.write(hb); err != nil {
		log.Warnf("write heartbeat message error: %v", err)
	}

	hb.Release()

	return true
}

// isClosed 是否已关闭
// @return @1 bool 连接状态是否为关闭
func (c *clientConn) isClosed() bool {
	return c.State() == network.ConnClosed
}
