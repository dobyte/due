package drpc

import (
	"bufio"
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
	"github.com/dobyte/due/v2/utils/xtime"
)

// session 表示一次连接的生命周期，读写协程通过它访问连接与上下文
type session struct {
	conn   *net.TCPConn
	ctx    context.Context
	cancel context.CancelFunc
}

type ClientConn struct {
	cli           *Client                             // 客户端
	mu            sync.Mutex                          // 保护 dialing/状态转换
	cond          *sync.Cond                          // 拨号完成条件变量
	rw            sync.RWMutex                        // 配对保护队列写入与关闭，避免向已关闭队列写入panic
	session       atomic.Pointer[session]             // 当前会话
	state         atomic.Int32                        // 连接状态
	queue         *queue.Queue[*buffer.NocopyBuffer]  // 消息队列
	retryBuf      atomic.Pointer[buffer.NocopyBuffer] // 写失败待重发的缓冲区
	pending       *pending                            // 等待队列
	dialing       bool                                // 是否正在拨号
	closed        atomic.Bool                         // 客户端是否已关闭
	lastFaultTime atomic.Int64                        // 上次故障时间
}

func newClientConn(cli *Client) *ClientConn {
	c := &ClientConn{}
	c.cli = cli
	c.state.Store(connClosed)
	c.queue = queue.NewQueue[*buffer.NocopyBuffer](int32(max(128, cli.opts.WriteQueueSize)), cli.opts.WriteTimeout)
	c.pending = newPending()
	c.cond = sync.NewCond(&c.mu)
	c.lastFaultTime.Store(xtime.Now().UnixNano())

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
		retry int
		delay time.Duration
	)

	for {
		if c.closed.Load() {
			return errors.ErrClientClosed
		}

		conn, err := net.DialTimeout(c.cli.addr.Network(), c.cli.addr.String(), c.cli.opts.DialTimeout)
		if err == nil {
			err = c.process(conn.(*net.TCPConn))
		}

		if err == nil {
			return nil
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

	// 同步握手：握手完成前不启动写协程、不进入业务 pending，隔离 seq=1 特殊序列号
	if err := c.handshake(s); err != nil {
		_ = conn.Close()
		s.cancel()
		return err
	}

	// 关闭期间到达的拨号结果直接废弃，避免已销毁连接被重新置为可用而产生僵尸会话
	if c.closed.Load() {
		_ = conn.Close()
		s.cancel()
		return errors.ErrClientClosed
	}

	c.mu.Lock()
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

	// 上报客户端实例级启动代次，重连重发时服务端可按 (insID, epoch, seq) 幂等去重
	buf := protocol.EncodeHandshakeReq(seq, c.cli.epoch, c.cli.opts.Kind, c.cli.opts.ID)

	// 覆盖握手写与读的整次 deadline；DialTimeout 未配置时使用兜底值，防止无响应的对端使握手永久阻塞
	timeout := c.cli.opts.DialTimeout
	if timeout <= 0 {
		timeout = defaultDialTimeout
	}
	_ = s.conn.SetDeadline(time.Now().Add(timeout))

	if _, err := s.conn.Write(buf.Bytes()); err != nil {
		buf.Release()
		return err
	}
	buf.Release()

	var (
		reader = bufio.NewReader(s.conn)
		header [4]byte
	)

	isHeartbeat, rt, rseq, data, err := protocol.ReadMessage(reader, &header)
	if err != nil {
		return err
	}

	// 清除读 deadline，交由正常读写循环管理
	_ = s.conn.SetDeadline(time.Time{})

	if isHeartbeat || rt != route.Handshake || rseq != seq {
		return errors.ErrInvalidMessage
	}

	code, err := protocol.DecodeHandshakeRes(data)
	if err != nil {
		return err
	}

	if err = codes.CodeToError(code); err != nil {
		return err
	}

	return nil
}

// send 发送消息
func (c *ClientConn) send(buf *buffer.NocopyBuffer) error {
	if err := c.doSend(buf); err != nil {
		buf.Release()
		return err
	}

	return nil
}

// doSend 执行发送
func (c *ClientConn) doSend(buf *buffer.NocopyBuffer) error {
	if c.closed.Load() {
		return errors.ErrClientClosed
	}

	switch c.state.Load() {
	case connClosed:
		if mode.IsReleaseMode() || mode.IsPreReleaseMode() {
			if xtime.Now().UnixNano()-c.lastFaultTime.Load() < c.cli.opts.FaultRecoveryTime.Nanoseconds() {
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

	// 与 destroy 中的 queue.Close 互斥，防止向已关闭的队列写入而 panic
	c.rw.RLock()
	err := c.queue.Write(buf)
	c.rw.RUnlock()

	return err
}

// call 调用
func (c *ClientConn) call(ctx context.Context, seq uint64, buf *buffer.NocopyBuffer) (buffer.Buffer, error) {
	call := make(chan buffer.Buffer, 1)

	// 复制请求数据，用于连接中断重连后重发，避免消息丢失
	data := append([]byte(nil), buf.Bytes()...)

	c.pending.store(seq, call, data)

	if err := c.send(buf); err != nil {
		c.pending.delete(seq)
		return nil, err
	}

	if c.cli.opts.CallTimeout > 0 {
		tctx, tcancel := context.WithTimeout(context.Background(), c.cli.opts.CallTimeout)
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
	var (
		reader = bufio.NewReaderSize(s.conn, 4096)
		header [4]byte
	)

	for {
		if s.ctx.Err() != nil {
			return
		}

		buf, err := protocol.ReaderBuffer(reader, &header)
		if err != nil {
			c.retry(s)
			return
		}

		if isHeartbeat, _, seq := protocol.ParseBuffer(buf.Bytes()); isHeartbeat {
			buf.Release()
		} else {
			if ok := c.pending.reply(seq, buf); !ok {
				buf.Release()
			}
		}
	}
}

// write 写入数据
func (c *ClientConn) write(s *session) {
	// 优先重发上次写失败的 Send 消息
	if buf := c.retryBuf.Load(); buf != nil {
		if c.doWrite(s, buf) {
			c.retryBuf.Store(nil)
		} else {
			return
		}
	}

	// 重连成功后优先重发所有未完成请求，保证请求不丢失
	c.resend(s)

	ticker := time.NewTicker(defaultHeartbeatInterval)
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

			c.queue.Done(false)

			if ok = c.doWrite(s, buf); !ok {
				return
			}
		}
	}
}

// resend 重发所有未完成（未收到响应）的请求
// 在重连后由写协程优先调用；写失败则继续触发重连
func (c *ClientConn) resend(s *session) {
	for _, entry := range c.pending.snapshot() {
		if entry == nil || len(entry.data) == 0 {
			continue
		}

		bs := net.Buffers{entry.data}

		if _, err := bs.WriteTo(s.conn); err != nil {
			c.retry(s)
			return
		}
	}
}

// doWrite 执行写入数据
// 写失败时：Send 消息（seq==0）保留到 retryBuf 待重连后重发；Call（seq>0）由 pending 重发兜底
func (c *ClientConn) doWrite(s *session, buf *buffer.NocopyBuffer) bool {
	var bs net.Buffers

	buf.Visit(func(node *buffer.NocopyNode) bool {
		bs = append(bs, node.Bytes())
		return true
	})

	if c.cli.opts.WriteTimeout > 0 {
		_ = s.conn.SetWriteDeadline(time.Now().Add(c.cli.opts.WriteTimeout))
	}

	_, err := bs.WriteTo(s.conn)

	if err != nil {
		if _, _, seq := protocol.ParseBuffer(buf.Bytes()); seq == 0 {
			c.retryBuf.Store(buf)
		} else {
			buf.Release()
		}

		c.retry(s)
		return false
	}

	buf.Release()

	return true
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

	c.lastFaultTime.Store(xtime.Now().UnixNano())

	if s != nil {
		if s.conn != nil {
			_ = s.conn.Close()
		}
		s.cancel()
	}
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

	c.lastFaultTime.Store(xtime.Now().UnixNano())

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

	if buf := c.retryBuf.Swap(nil); buf != nil {
		buf.Release()
	}
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

func (c *ClientConn) discard(seq uint64, call chan buffer.Buffer) {
	c.pending.delete(seq)

	select {
	case buf, ok := <-call:
		if ok && buf != nil {
			buf.Release()
		}
	default:
	}
}
