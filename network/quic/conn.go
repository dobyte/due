package quic

import (
	"bufio"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/utils/xnet"
	"github.com/quic-go/quic-go"
)

type connOptions struct {
	server                                                          bool
	queueSize                                                       int
	writeTimeout, closeTimeout, heartbeatInterval, authorizeTimeout time.Duration
	heartbeatMechanism                                              HeartbeatMechanism
	connect                                                         network.ConnectHandler
	disconnect                                                      network.DisconnectHandler
	receive                                                         network.ReceiveHandler
	heartbeat                                                       network.HeartbeatHandler
}

// conn is never reused: references retained by application code remain closed.
type conn struct {
	id                         int64
	uid                        atomic.Int64
	attr                       attr
	state                      atomic.Int32
	qc                         *quic.Conn
	stream                     *quic.Stream
	output                     *bufferWriter
	opts                       connOptions
	queue                      chan buffer.Buffer
	queueMu                    sync.RWMutex
	sealOnce                   sync.Once
	closing                    chan struct{}
	abort                      chan struct{}
	done                       chan struct{}
	mu                         sync.Mutex
	authorizeTimer, closeTimer *time.Timer
	authorizeGeneration        uint64
	lastHeartbeatTime          atomic.Int64
	heartbeatReply             chan struct{}
	onClosed                   func()
}

var _ network.Conn = (*conn)(nil)

func newConn(id int64, qc *quic.Conn, stream *quic.Stream, opts connOptions) *conn {
	c := &conn{id: id, qc: qc, stream: stream, opts: opts,
		queue:   make(chan buffer.Buffer, max(1, opts.queueSize)),
		closing: make(chan struct{}), abort: make(chan struct{}), done: make(chan struct{}),
		heartbeatReply: make(chan struct{}, 1)}
	c.state.Store(int32(network.ConnOpened))
	c.output = newBufferWriter(stream)
	c.lastHeartbeatTime.Store(time.Now().UnixNano())
	return c
}

// start runs connection callbacks in read order, with the writer already available.
func (c *conn) start() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); c.write() }()
	go func() {
		defer wg.Done()
		c.mu.Lock()
		c.checkAuthorizeLocked()
		c.mu.Unlock()
		if c.opts.connect != nil {
			c.opts.connect(c)
		}
		c.read()
	}()
	go func() {
		<-c.qc.Context().Done()
		_ = c.forceClose()
		wg.Wait()
		close(c.done)
		if c.opts.disconnect != nil {
			c.opts.disconnect(c)
		}
	}()
}

// ID returns the immutable connection ID.
func (c *conn) ID() int64 { return c.id }

// UID returns the bound user ID, or zero if unbound.
func (c *conn) UID() int64 { return c.uid.Load() }

// Attr returns the connection attributes.
func (c *conn) Attr() network.Attr { return &c.attr }

// State returns the current connection state.
func (c *conn) State() network.ConnState { return network.ConnState(c.state.Load()) }

// Bind binds a user and cancels authorization expiry.
func (c *conn) Bind(uid int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.State() != network.ConnOpened {
		return c.checkState()
	}
	c.uid.Store(uid)
	c.stopAuthorizeLocked()
	return nil
}

// Unbind clears the user and restarts authorization expiry.
func (c *conn) Unbind() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.State() != network.ConnOpened {
		return c.checkState()
	}
	c.uid.Store(0)
	c.checkAuthorizeLocked()
	return nil
}

// Push transfers ownership of buf only when it returns nil.
// A failed enqueue leaves ownership with the caller.
func (c *conn) Push(buf buffer.Buffer) error {
	if buf == nil || buf.Len() == 0 {
		return errors.ErrInvalidMessage
	}
	c.queueMu.RLock()
	defer c.queueMu.RUnlock()
	if err := c.checkState(); err != nil {
		return err
	}
	select {
	case c.queue <- buf:
		return nil
	default:
	}
	var timeout <-chan time.Time
	if c.opts.writeTimeout > 0 {
		timer := time.NewTimer(c.opts.writeTimeout)
		defer timer.Stop()
		timeout = timer.C
	}
	select {
	case <-c.closing:
		return c.checkState()
	case c.queue <- buf:
		return nil
	case <-timeout:
		return errors.ErrWriteTimeout
	}
}

func (c *conn) checkState() error {
	switch c.State() {
	case network.ConnClosed:
		return errors.ErrConnectionClosed
	case network.ConnHanged:
		return errors.ErrConnectionHanged
	default:
		return nil
	}
}

// Close initiates shutdown without waiting for application callbacks.
// Graceful shutdown drains accepted messages and sends FIN, retaining the QUIC
// transport for closeTimeout to allow delivery and retransmission. This is bounded
// best effort, not an application-level delivery acknowledgement. Force interrupts
// I/O immediately. OnDisconnect runs after both I/O workers have exited.
func (c *conn) Close(force ...bool) error {
	if len(force) > 0 && force[0] {
		return c.forceClose()
	}
	c.mu.Lock()
	if c.State() != network.ConnOpened {
		c.mu.Unlock()
		return errors.ErrConnectionNotOpened
	}
	c.state.Store(int32(network.ConnHanged))
	close(c.closing)
	c.stopAuthorizeLocked()
	c.closeTimer = time.AfterFunc(c.opts.closeTimeout, func() { _ = c.forceClose() })
	c.mu.Unlock()
	c.sealQueue()
	return nil
}

func (c *conn) sealQueue() {
	c.sealOnce.Do(func() {
		c.queueMu.Lock()
		close(c.queue)
		c.queueMu.Unlock()
	})
}

func (c *conn) forceClose() error {
	c.mu.Lock()
	if c.State() == network.ConnClosed {
		c.mu.Unlock()
		return errors.ErrConnectionClosed
	}
	wasOpened := c.State() == network.ConnOpened
	c.state.Store(int32(network.ConnClosed))
	if wasOpened {
		close(c.closing)
	}
	close(c.abort)
	c.stopAuthorizeLocked()
	if c.closeTimer != nil {
		c.closeTimer.Stop()
	}
	c.mu.Unlock()
	// Interrupt a flow-control-blocked Write before waiting for enqueue readers.
	err := c.qc.CloseWithError(0, "closed")
	c.sealQueue()
	if c.onClosed != nil {
		c.onClosed()
	}
	return err
}

func (c *conn) stopAuthorizeLocked() {
	c.authorizeGeneration++
	if c.authorizeTimer != nil {
		c.authorizeTimer.Stop()
		c.authorizeTimer = nil
	}
}

func (c *conn) checkAuthorizeLocked() {
	c.stopAuthorizeLocked()
	if c.opts.authorizeTimeout <= 0 || c.State() != network.ConnOpened {
		return
	}
	generation := c.authorizeGeneration
	c.authorizeTimer = time.AfterFunc(c.opts.authorizeTimeout, func() {
		c.mu.Lock()
		if generation != c.authorizeGeneration || c.uid.Load() != 0 || c.State() != network.ConnOpened {
			c.mu.Unlock()
			return
		}
		// Commit expiry under the same lock as Bind and Unbind.
		c.state.Store(int32(network.ConnHanged))
		close(c.closing)
		c.mu.Unlock()
		_ = c.forceClose()
	})
}

// LocalAddr returns the local endpoint while the connection remains open.
func (c *conn) LocalAddr() (net.Addr, error) {
	if c.State() == network.ConnClosed {
		return nil, errors.ErrConnectionClosed
	}
	return c.qc.LocalAddr(), nil
}

// RemoteAddr returns the remote endpoint while the connection remains open.
func (c *conn) RemoteAddr() (net.Addr, error) {
	if c.State() == network.ConnClosed {
		return nil, errors.ErrConnectionClosed
	}
	return c.qc.RemoteAddr(), nil
}

// LocalIP returns the local IP address.
func (c *conn) LocalIP() (string, error) {
	a, err := c.LocalAddr()
	if err != nil {
		return "", err
	}
	return xnet.ExtractIP(a)
}

// RemoteIP returns the remote IP address.
func (c *conn) RemoteIP() (string, error) {
	a, err := c.RemoteAddr()
	if err != nil {
		return "", err
	}
	return xnet.ExtractIP(a)
}

func (c *conn) read() {
	reader := bufio.NewReaderSize(c.stream, 4096)
	for c.State() != network.ConnClosed {
		isHeartbeat, heartbeatTime, buf, err := packet.Read(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				_ = c.Close()
			} else {
				_ = c.forceClose()
			}
			return
		}
		if c.State() == network.ConnClosed {
			if buf != nil {
				buf.Release()
			}
			return
		}
		if c.opts.heartbeatInterval > 0 {
			c.lastHeartbeatTime.Store(time.Now().UnixNano())
		}
		if isHeartbeat {
			if c.opts.server && c.opts.heartbeatMechanism == RespHeartbeat && c.State() == network.ConnOpened {
				// Coalesce replies rather than blocking the reader on a full data queue.
				select {
				case c.heartbeatReply <- struct{}{}:
				default:
				}
			}
			if c.opts.heartbeat != nil {
				c.opts.heartbeat(c, heartbeatTime)
			}
		} else if buf != nil {
			if c.opts.receive != nil && buf.Len() > 0 {
				c.opts.receive(c, buf)
			} else {
				buf.Release()
			}
		}
	}
}

func (c *conn) write() {
	defer func() {
		// The queue must be sealed before draining to synchronize all producers.
		c.sealQueue()
		for buf := range c.queue {
			buf.Release()
		}
	}()
	var tick <-chan time.Time
	if c.opts.heartbeatInterval > 0 {
		ticker := time.NewTicker(c.opts.heartbeatInterval)
		defer ticker.Stop()
		tick = ticker.C
	}
	for {
		select {
		case <-c.abort:
			return
		case buf, ok := <-c.queue:
			if !ok {
				if err := c.stream.Close(); err != nil {
					_ = c.forceClose()
				}
				return
			}
			err := c.writeBuffer(buf)
			buf.Release()
			if err != nil {
				_ = c.forceClose()
				return
			}
		case <-c.heartbeatReply:
			if err := c.writeHeartbeat(); err != nil {
				_ = c.forceClose()
				return
			}
		case now := <-tick:
			if c.State() != network.ConnOpened {
				continue
			}
			if now.UnixNano()-c.lastHeartbeatTime.Load() > int64(2*c.opts.heartbeatInterval) {
				_ = c.forceClose()
				return
			}
			if !c.opts.server || c.opts.heartbeatMechanism == TickHeartbeat {
				if err := c.writeHeartbeat(); err != nil {
					_ = c.forceClose()
					return
				}
			}
		}
	}
}

func (c *conn) writeHeartbeat() error {
	hb := packet.PackHeartbeat(c.opts.server)
	// Release is a no-op for the default packer's static heartbeat and recycles
	// its timestamped heartbeat buffer.
	defer hb.Release()
	return c.writeBuffer(hb)
}

func (c *conn) writeBuffer(buf buffer.Buffer) error {
	if c.opts.writeTimeout > 0 {
		if err := c.stream.SetWriteDeadline(time.Now().Add(c.opts.writeTimeout)); err != nil {
			return err
		}
	}
	return c.output.write(buf)
}

// writeBuffer avoids flattening composite buffers and handles short writes.
func writeBuffer(writer io.Writer, buf buffer.Buffer) error {
	if buf.Nodes() == 1 {
		return writeAll(writer, buf.Bytes())
	}
	return newBufferWriter(writer).write(buf)
}

// bufferWriter reuses the VisitBytes callback rather than allocating it per packet.
// Only the connection's write worker uses this object.
type bufferWriter struct {
	writer io.Writer
	err    error
	visit  func([]byte) bool
}

func newBufferWriter(writer io.Writer) *bufferWriter {
	w := &bufferWriter{writer: writer}
	w.visit = w.writePart
	return w
}

func (w *bufferWriter) write(buf buffer.Buffer) error {
	if buf.Nodes() == 1 {
		return writeAll(w.writer, buf.Bytes())
	}
	w.err = nil
	buf.VisitBytes(w.visit)
	return w.err
}

func (w *bufferWriter) writePart(b []byte) bool {
	w.err = writeAll(w.writer, b)
	return w.err == nil
}

func writeAll(writer io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := writer.Write(b)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(b) {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
