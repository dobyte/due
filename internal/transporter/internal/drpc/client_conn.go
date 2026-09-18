package drpc

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
)

// session 表示一次连接的生命周期，读写协程通过它访问连接与上下文
type session struct {
	ctx        context.Context
	cancel     context.CancelFunc
	conn       *net.TCPConn
	reader     *reader
	dueBuffers []*buffer.NocopyBuffer // 待写入的消息缓冲对象集合（写协程独享）
	netBuffers net.Buffers            // 待写入的字节切片集合（写协程独享）
}

type ClientConn struct {
	cli           *Client                            // 客户端
	epoch         uint64                             // 连接时间戳
	mu            sync.Mutex                         // 保护 dialing/状态转换
	cond          *sync.Cond                         // 拨号完成条件变量
	rw            sync.RWMutex                       // 配对保护队列写入与关闭，避免向已关闭队列写入panic
	session       atomic.Pointer[session]            // 当前会话
	state         atomic.Int32                       // 连接状态
	queue         *queue.Queue[*buffer.NocopyBuffer] // 消息队列
	pending       *pending                           // 等待队列
	dialing       bool                               // 是否正在拨号
	closed        atomic.Bool                        // 客户端是否已关闭
	lastFaultTime atomic.Int64                       // 上次故障时间
}

func newClientConn(cli *Client) *ClientConn {
	c := &ClientConn{}
	c.cli = cli
	c.epoch = cli.doGenEpoch()
	c.state.Store(connClosed)
	c.queue = queue.NewQueue[*buffer.NocopyBuffer](int32(max(128, cli.opts.WriteQueueSize)), cli.opts.WriteTimeout)
	c.pending = newPending()
	c.cond = sync.NewCond(&c.mu)
	c.lastFaultTime.Store(time.Now().UnixNano())

	return c
}

// dial 建立连接；若已有拨号进行中，则等待其完成
func (c *ClientConn) dial() error {
	c.mu.Lock()

	if c.state.Load() == connOpened {
		c.mu.Unlock()
		return nil
	}

	// 已有拨号在进行，等待其完成后再判定结果
	for c.dialing {
		c.cond.Wait()
	}

	if c.state.Load() == connOpened {
		c.mu.Unlock()
		return nil
	}

	c.dialing = true
	c.mu.Unlock()

	// 在锁外执行阻塞的网络拨号，避免长时间持锁
	err := c.doDial()

	c.mu.Lock()
	c.dialing = false
	c.cond.Broadcast()
	c.mu.Unlock()

	return err
}

// doDial 执行拨号
// 拨号失败与握手失败统一按退避策略重试，避免握手失败后直接放弃
func (c *ClientConn) doDial() error {
	var (
		err   error
		conn  net.Conn
		retry int
		delay time.Duration
	)

	for {
		if c.closed.Load() {
			return errors.ErrClientClosed
		}

		if conn, err = net.DialTimeout(c.cli.addr.Network(), c.cli.addr.String(), c.cli.opts.DialTimeout); err == nil {
			if err = c.process(conn.(*net.TCPConn)); err == nil {
				return nil
			}
		}

		if c.cli.opts.DialRetryTimes >= 0 {
			retry++

			if retry > c.cli.opts.DialRetryTimes {
				c.close()
				return err
			}
		}

		if delay == 0 {
			delay = 5 * time.Millisecond
		} else {
			delay *= 2
		}

		if delay > time.Second {
			delay = time.Second
		}

		time.Sleep(delay)
	}
}

// process 处理连接
func (c *ClientConn) process(conn *net.TCPConn) error {
	s := &session{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.conn = conn
	s.conn.SetNoDelay(true)
	s.reader = newReader(s.conn)

	// 同步握手：握手完成前不启动写协程、不进入业务 pending，隔离 seq=1 特殊序列号
	if err := c.handshake(s); err != nil {
		_ = conn.Close()
		s.cancel()
		return err
	}

	c.mu.Lock()

	// 关闭期间到达的拨号结果直接废弃，避免已销毁连接被重新置为可用而产生僵尸会话
	if c.closed.Load() {
		c.mu.Unlock()
		_ = conn.Close()
		s.cancel()
		return errors.ErrClientClosed
	}

	c.session.Store(s)
	c.state.Store(connOpened)
	c.mu.Unlock()

	go c.read(s)
	go c.write(s)

	return nil
}

// handshake 握手
// 同步完成握手交互与响应校验，不占用业务 pending 分片
func (c *ClientConn) handshake(s *session) error {
	const seq = uint64(1)

	req := protocol.EncodeHandshakeReq(seq, c.cli.opts.Kind, c.cli.opts.ID, c.epoch)
	defer req.Release()

	if c.cli.opts.DialTimeout > 0 {
		_ = s.conn.SetDeadline(time.Now().Add(c.cli.opts.DialTimeout))
	}

	if _, err := s.conn.Write(req.Bytes()); err != nil {
		return err
	}

	isHeartbeat, rt, rseq, res, err := s.reader.read()
	if err != nil {
		return err
	}

	if isHeartbeat {
		return errors.ErrInvalidMessage
	}

	defer res.Release()

	if rt != route.Handshake || rseq != seq {
		return errors.ErrInvalidMessage
	}

	_ = s.conn.SetDeadline(time.Time{})

	code, err := protocol.DecodeHandshakeRes(res)
	if err != nil {
		return err
	}

	if err = codes.CodeToError(code); err != nil {
		return err
	}

	return nil
}

func (c *ClientConn) doPush(buf *buffer.NocopyBuffer) error {
	if c.closed.Load() {
		return errors.ErrClientClosed
	}

	switch c.state.Load() {
	case connClosed:
		if mode.IsReleaseMode() || mode.IsPreReleaseMode() {
			if time.Now().UnixNano()-c.lastFaultTime.Load() < c.cli.opts.FaultRecoveryTime.Nanoseconds() {
				return errors.ErrConnectionClosed
			}
		}

		if err := c.dial(); err != nil {
			return err
		}
	case connHanged:
		if err := c.wait(); err != nil {
			return err
		}
	}

	c.rw.RLock()
	err := c.queue.Write(buf)
	c.rw.RUnlock()

	return err
}

// push 发送消息
func (c *ClientConn) push(buf *buffer.NocopyBuffer) error {
	if err := c.doPush(buf); err != nil {
		buf.Release()
		return err
	}

	return nil
}

// call 调用
func (c *ClientConn) call(ctx context.Context, seq uint64, buf *buffer.NocopyBuffer) (buffer.Buffer, error) {
	call := make(chan *buffer.Bytes, 1)

	c.pending.store(seq, call)

	if err := c.push(buf); err != nil {
		c.pending.delete(seq)
		return nil, err
	}

	if c.cli.opts.CallTimeout > 0 {
		tctx, tcancel := context.WithTimeout(ctx, c.cli.opts.CallTimeout)
		defer tcancel()

		select {
		case <-ctx.Done():
			c.discard(seq, call)
			return nil, ctx.Err()
		case <-tctx.Done():
			c.discard(seq, call)
			return nil, tctx.Err()
		case res, ok := <-call:
			if !ok {
				return nil, errors.ErrConnectionHanged
			}
			close(call)
			return res, nil
		}
	} else {
		select {
		case <-ctx.Done():
			c.discard(seq, call)
			return nil, ctx.Err()
		case res, ok := <-call:
			if !ok {
				return nil, errors.ErrConnectionHanged
			}
			close(call)
			return res, nil
		}
	}
}

// read 读取数据
func (c *ClientConn) read(s *session) {
	for {
		if s.ctx.Err() != nil {
			return
		}

		isHeartbeat, _, seq, buf, err := s.reader.read()
		if err != nil {
			c.retry(s)
			return
		}

		if isHeartbeat {
			continue
		}

		if !c.pending.reply(seq, buf) {
			buf.Release()
		}
	}
}

// write 写入数据
func (c *ClientConn) write(s *session) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if c.cli.opts.WriteTimeout > 0 {
				_ = s.conn.SetWriteDeadline(time.Now().Add(c.cli.opts.WriteTimeout))
			}

			if _, err := s.conn.Write(protocol.Heartbeat()); err != nil {
				log.Warnf("write heartbeat message error: %v", err)
				c.retry(s)
				return
			}
		case buf, ok := <-c.queue.Read():
			if !ok {
				return
			}

			if err := c.doBatchWrite(s, buf); err != nil {
				c.retry(s)
				return
			}
		}
	}
}

// doBatchWrite 批量写入消息
// 从写队列批量取出任务，收集字节后通过net.Buffers一次性下发，减少系统调用次数
// @param conn net.Conn TCP连接
// @param first buffer.Buffer 首个已取出的任务
func (c *ClientConn) doBatchWrite(s *session, first *buffer.NocopyBuffer) (err error) {
	closeSig := first.Len() == 0

	c.queue.Done(closeSig)

	if closeSig {
		first.Release()
		return
	}

	s.dueBuffers = s.dueBuffers[:0]
	s.dueBuffers = append(s.dueBuffers, first)

	for len(s.dueBuffers) < maxBatchWriteNum {
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

			s.dueBuffers = append(s.dueBuffers, buf)
		default:
			goto OVER
		}
	}

OVER:
	s.netBuffers = s.netBuffers[:0]

	for _, buf := range s.dueBuffers {
		buf.VisitBytes(func(bytes []byte) bool {
			s.netBuffers = append(s.netBuffers, bytes)
			return true
		})
	}

	if len(s.netBuffers) > 0 {
		if c.cli.opts.WriteTimeout > 0 {
			_ = s.conn.SetWriteDeadline(time.Now().Add(c.cli.opts.WriteTimeout))
		}

		_, err = s.netBuffers.WriteTo(s.conn)
	}

	for _, buf := range s.dueBuffers {
		buf.Release()
	}

	s.netBuffers = s.netBuffers[:0]
	s.dueBuffers = s.dueBuffers[:0]

	if err != nil && !errors.Is(err, net.ErrClosed) {
		log.Errorf("write message error: %v", err)
	}

	return
}

// retry 重试拨号
// 仅当传入的会话仍是当前会话时才触发重连，避免旧读写协程误伤新连接
func (c *ClientConn) retry(s *session) {
	if c.closed.Load() {
		return
	}

	c.mu.Lock()
	if c.session.Load() != s || c.state.Load() != connOpened {
		c.mu.Unlock()
		return
	}
	c.state.Store(connHanged)
	c.mu.Unlock()

	_ = s.conn.Close()
	s.cancel()

	if err := c.dial(); err != nil {
		log.Warnf("retry dial failed: %v", err)
	}
}

// close 关闭连接
// 拨号重试耗尽时调用，将连接置为关闭态。队列保持打开以支持后续重连补发消息，
// 但需主动唤醒所有等待中的调用，避免其阻塞至调用超时
func (c *ClientConn) close() {
	c.mu.Lock()
	if c.state.Load() == connClosed {
		c.mu.Unlock()
		return
	}
	c.state.Store(connClosed)
	s := c.session.Load()
	c.session.Store(nil)
	c.cond.Broadcast()
	c.mu.Unlock()

	c.lastFaultTime.Store(time.Now().UnixNano())

	if s != nil {
		if s.conn != nil {
			_ = s.conn.Close()
		}
		s.cancel()
	}

	c.pending.closeAll()
}

// destroy 销毁连接
// 关闭会话、关闭消息队列并释放积压的消息，彻底回收连接资源
func (c *ClientConn) destroy() {
	c.mu.Lock()
	s := c.session.Load()
	c.session.Store(nil)
	c.state.Store(connClosed)
	c.cond.Broadcast()
	c.mu.Unlock()

	c.lastFaultTime.Store(time.Now().UnixNano())

	if s != nil {
		if s.conn != nil {
			_ = s.conn.Close()
		}
		s.cancel()
	}

	// 与 doSend 中的 queue.Write 互斥，防止向已关闭的队列写入而 panic
	c.rw.Lock()
	c.queue.Close()
	c.rw.Unlock()

	for buf := range c.queue.Read() {
		if buf != nil {
			buf.Release()
		}
	}

	c.pending.closeAll()
}

// wait 等待重连
func (c *ClientConn) wait() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for c.state.Load() == connHanged {
		c.cond.Wait()
	}

	if c.state.Load() == connOpened {
		return nil
	}

	return errors.ErrConnectionClosed
}

func (c *ClientConn) discard(seq uint64, call chan *buffer.Bytes) {
	c.pending.delete(seq)

	select {
	case buf, ok := <-call:
		if ok && buf != nil {
			buf.Release()
		}
	default:
	}
}
