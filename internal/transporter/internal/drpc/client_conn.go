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
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
	"github.com/dobyte/due/v2/utils/xtime"
)

// session 表示一次连接的生命周期，读写协程通过它访问连接与上下文
type session struct {
	conn   net.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

type ClientConn struct {
	cli           *Client                            // 客户端
	mu            sync.Mutex                         // 保护 dialing/状态转换
	cond          *sync.Cond                         // 拨号完成条件变量
	session       atomic.Pointer[session]            // 当前会话
	state         atomic.Int32                       // 连接状态
	queue         *queue.Queue[*buffer.NocopyBuffer] // 消息队列
	pending       *pending                           // 等待队列
	dialing       bool                               // 是否正在拨号
	lastFaultTime atomic.Int64                       // 上次故障时间
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
		conn, err := net.DialTimeout(c.cli.addr.Network(), c.cli.addr.String(), c.cli.opts.DialTimeout)
		if err == nil {
			err = c.process(conn)
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
func (c *ClientConn) process(conn net.Conn) error {
	ctx, cancel := context.WithCancel(context.Background())
	s := &session{conn: conn, ctx: ctx, cancel: cancel}

	c.mu.Lock()
	c.session.Store(s)
	c.state.Store(connOpened)
	c.mu.Unlock()

	conn.(*net.TCPConn).SetNoDelay(true)

	go c.read(s)

	if err := c.handshake(s); err != nil {
		c.close()
		return err
	}

	go c.write(s)

	return nil
}

// handshake 握手
func (c *ClientConn) handshake(s *session) error {
	var (
		seq  = uint64(1)
		buf  = protocol.EncodeHandshakeReq(seq, c.cli.opts.Kind, c.cli.opts.ID)
		call = make(chan buffer.Buffer, 1)
	)

	c.pending.store(seq, call)

	if _, err := s.conn.Write(buf.Bytes()); err != nil {
		buf.Release()
		c.discard(seq, call)
		return err
	}
	buf.Release()

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		c.discard(seq, call)
		return ctx.Err()
	case buf := <-call:
		buf.Release()
		return nil
	}
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

	return c.queue.Write(buf)
}

// call 调用
func (c *ClientConn) call(ctx context.Context, seq uint64, buf *buffer.NocopyBuffer) (buffer.Buffer, error) {
	call := make(chan buffer.Buffer, 1)
	c.pending.store(seq, call)

	if err := c.send(buf); err != nil {
		c.pending.delete(seq)
		return nil, err
	}

	s := c.session.Load()

	if s == nil {
		c.pending.delete(seq)
		return nil, errors.ErrConnectionClosed
	}

	if c.cli.opts.CallTimeout > 0 {
		tctx, tcancel := context.WithTimeout(s.ctx, c.cli.opts.CallTimeout)
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
	ticker := time.NewTicker(defaultHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
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

// doWrite 执行写入数据
func (c *ClientConn) doWrite(s *session, buf *buffer.NocopyBuffer) bool {
	var bs net.Buffers

	buf.Visit(func(node *buffer.NocopyNode) bool {
		bs = append(bs, node.Bytes())
		return true
	})

	_, err := bs.WriteTo(s.conn)

	buf.Release()

	if err != nil {
		c.retry(s)
		return false
	}

	return true
}

// retry 重试拨号
// 仅当传入的会话仍是当前会话时才触发重连，避免旧读写协程误伤新连接
func (c *ClientConn) retry(s *session) {
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

	c.queue.Close()

	for buf := range c.queue.Read() {
		if buf != nil {
			buf.Release()
		}
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
