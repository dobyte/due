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
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
	"github.com/dobyte/due/v2/utils/xtime"
)

type ClientConn struct {
	cli           *Client                // 客户端
	rw            sync.RWMutex           // 读写锁
	conn          net.Conn               // 连接
	state         atomic.Int32           // 连接状态
	total         atomic.Int32           // 总消息数
	queue         *queue.Queue[*message] // 消息队列
	pending       *pending               // 等待队列
	failure       chan struct{}          // 重试失败通道
	success       chan struct{}          // 重试成功通道
	wg1           *sync.WaitGroup        // 读等待组
	wg2           *sync.WaitGroup        // 写等待组
	ctx           context.Context        // 上下文
	cancel        context.CancelFunc     // 取消函数
	lastFaultTime atomic.Int64           // 上次故障时间
}

func newClientConn(cli *Client) *ClientConn {
	c := &ClientConn{}
	c.cli = cli
	c.state.Store(def.ConnClosed)
	c.queue = queue.NewQueue[*message](int32(max(128, cli.opts.WriteQueueSize)), cli.opts.WriteTimeout)
	c.pending = newPending()
	c.failure = make(chan struct{})
	c.success = make(chan struct{})
	c.lastFaultTime.Store(xtime.Now().UnixNano())

	return c
}

// 拨号
func (c *ClientConn) dial() error {
	c.rw.Lock()
	defer c.rw.Unlock()

	if c.state.Load() == def.ConnOpened {
		return nil
	}

	if err := c.doDial(); err != nil {
		close(c.failure)
		c.failure = make(chan struct{})

		return err
	} else {
		close(c.success)
		c.success = make(chan struct{})

		return nil
	}
}

// 执行拨号
func (c *ClientConn) doDial() error {
	var (
		retry int
		delay time.Duration
	)

	for {
		conn, err := net.DialTimeout(c.cli.addr.Network(), c.cli.addr.String(), c.cli.opts.DialTimeout)
		if err != nil {
			retry++

			if retry >= c.cli.opts.DialRetryTimes {
				c.close()
				return err
			} else {
				if delay == 0 {
					delay = 5 * time.Millisecond
				} else {
					delay *= 2
				}

				if delay > time.Second {
					delay = time.Second
				}

				time.Sleep(delay)
				continue
			}
		}

		return c.process(conn)
	}
}

// 处理连接
func (c *ClientConn) process(conn net.Conn) error {
	c.conn = conn
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.state.Store(def.ConnOpened)
	c.conn.(*net.TCPConn).SetNoDelay(true)
	c.wg1 = &sync.WaitGroup{}
	c.wg1.Go(func() { c.read(conn) })

	err := c.handshake(conn)
	if err != nil {
		c.close()
	} else {
		c.wg2 = &sync.WaitGroup{}
		c.wg2.Go(func() { c.write(conn) })
	}

	return err
}

// 握手
func (c *ClientConn) handshake(conn net.Conn) error {
	var (
		seq  = uint64(1)
		buf  = protocol.EncodeHandshakeReq(seq, c.cli.opts.Kind, c.cli.opts.ID)
		call = make(chan buffer.Buffer)
	)

	c.pending.store(seq, call)

	if _, err := conn.Write(buf.Bytes()); err != nil {
		buf.Release()

		close(call)

		c.pending.delete(seq)

		return err
	} else {
		buf.Release()
	}

	ctx, cancel := context.WithTimeout(c.ctx, 3*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		c.pending.delete(seq)

		return ctx.Err()
	case buf := <-call:
		buf.Release()

		return nil
	}
}

// 发送消息
func (c *ClientConn) send(msg *message) error {
	switch c.state.Load() {
	case def.ConnClosed:
		if mode.IsReleaseMode() || mode.IsPreReleaseMode() {
			if xtime.Now().UnixNano()-c.lastFaultTime.Load() < c.cli.opts.FaultRecoveryTime.Nanoseconds() {
				return errors.ErrConnectionClosed
			}
		}

		if err := c.dial(); err != nil {
			return err
		}
	case def.ConnHanged:
		if err := c.wait(); err != nil {
			return err
		}
	}

	c.rw.RLock()
	defer c.rw.RUnlock()

	return c.queue.Write(msg)
}

// 读取数据
func (c *ClientConn) read(conn net.Conn) {
	var (
		reader = bufio.NewReaderSize(conn, 4096)
		header [4]byte
	)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			buf, err := protocol.ReaderBuffer(reader, &header)
			if err != nil {
				c.retry(conn)
				return
			}

			if isHeartbeat, _, seq := protocol.ParseBuffer(buf.Bytes()); isHeartbeat {
				buf.Release()
			} else {
				if call, ok := c.pending.extract(seq); ok {
					call <- buf
				} else {
					buf.Release()
				}
			}
		}
	}
}

// 写入数据
func (c *ClientConn) write(conn net.Conn) {
	ticker := time.NewTicker(def.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if _, err := conn.Write(protocol.Heartbeat()); err != nil {
				log.Warnf("write heartbeat message error: %v", err)
				c.retry(conn)
				return
			}
		case msg, ok := <-c.queue.Read(): // 有序队列
			if !ok {
				return
			}

			if c.cli.opts.WriteTimeout > 0 {
				c.total.Add(-1)
			}

			if ok = c.doWrite(conn, msg); !ok {
				return
			}
		}
	}
}

// 执行写入数据
func (c *ClientConn) doWrite(conn net.Conn, msg *message) bool {
	if msg.seq != 0 {
		if !msg.state.CompareAndSwap(statePending, stateSent) {
			c.cli.release(msg, true)
			return false
		}

		c.pending.store(msg.seq, msg.call)
	}

	ok := msg.buf.Visit(func(node *buffer.NocopyNode) bool {
		if _, err := conn.Write(node.Bytes()); err != nil {
			return false
		} else {
			return true
		}
	})

	c.cli.release(msg)

	if !ok {
		c.retry(conn)
	}

	return ok
}

// 重试拨号
func (c *ClientConn) retry(conn net.Conn) {
	if !c.state.CompareAndSwap(def.ConnOpened, def.ConnHanged) {
		return
	}

	_ = conn.Close()

	if c.cancel != nil {
		c.cancel()
	}

	if err := c.dial(); err != nil {
		log.Warnf("retry dial failed: %v", err)
	}
}

// 关闭连接
func (c *ClientConn) close() {
	if c.state.Swap(def.ConnClosed) == def.ConnClosed {
		return
	}

	c.lastFaultTime.Store(xtime.Now().UnixNano())

	if c.conn != nil {
		_ = c.conn.Close()
	}

	if c.cancel != nil {
		c.cancel()
	}
}

// 等待重连
func (c *ClientConn) wait() error {
	c.rw.RLock()
	defer c.rw.RUnlock()

	switch c.state.Load() {
	case def.ConnOpened:
		return nil
	case def.ConnHanged:
		select {
		case <-c.failure:
			return errors.ErrConnectionClosed
		case <-c.success:
			return nil
		}
	}

	return errors.ErrConnectionClosed
}

// 删除发送消息
func (c *ClientConn) delete(msg *message) {
	if !msg.state.CompareAndSwap(statePending, stateCanceled) {
		c.pending.delete(msg.seq)
	}
}
