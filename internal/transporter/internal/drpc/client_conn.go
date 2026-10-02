package drpc

import (
	"context"
	"math/rand/v2"
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

// session represents the lifetime of one connection; the read and write goroutines use it to
// access the connection and its context.
type session struct {
	ctx           context.Context
	cancel        context.CancelFunc
	conn          *net.TCPConn
	reader        *reader
	dueBuffers    []*buffer.NocopyBuffer // Message buffers pending write (owned by the write goroutine)
	netBuffers    net.Buffers            // Byte slices pending write (owned by the write goroutine)
	writeDeadline time.Time              // Write deadline (owned by the write goroutine)
}

type ClientConn struct {
	cli           *Client                            // Client
	epoch         uint64                             // Connection epoch
	mu            sync.Mutex                         // Guards dialing and state transitions
	cond          *sync.Cond                         // Condition variable for dial completion
	rw            sync.RWMutex                       // Serializes queue writes against close, preventing a write to a closed queue from panicking
	session       atomic.Pointer[session]            // Current session
	state         atomic.Int32                       // Connection state
	queue         *queue.Queue[*buffer.NocopyBuffer] // Message queue
	pending       *pending                           // Pending calls
	dialing       bool                               // Whether a dial is in progress
	closed        atomic.Bool                        // Whether the client has been closed
	lastFaultTime atomic.Int64                       // Time of the last fault
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

// dial establishes a connection, waiting for an in-flight dial to finish first.
func (c *ClientConn) dial() error {
	c.mu.Lock()

	if c.state.Load() == connOpened {
		c.mu.Unlock()
		return nil
	}

	// A dial is already in progress; wait for it to finish before checking the result.
	for c.dialing {
		c.cond.Wait()
	}

	if c.state.Load() == connOpened {
		c.mu.Unlock()
		return nil
	}

	c.dialing = true
	c.mu.Unlock()

	// Perform the blocking network dial outside the lock to avoid holding it for a long time.
	err := c.doDial()

	c.mu.Lock()
	c.dialing = false
	c.cond.Broadcast()
	c.mu.Unlock()

	return err
}

// doDial performs the dialing loop.
//
// Both dial failures and handshake failures are retried with backoff, so that a handshake failure
// does not give up immediately.
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

// process processes a newly dialed connection.
func (c *ClientConn) process(conn *net.TCPConn) error {
	s := &session{}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.conn = conn
	s.conn.SetNoDelay(true)
	s.reader = newReader(s.conn)

	// Synchronous handshake: the write goroutine is not started and no business call enters
	// pending before the handshake completes, isolating the special sequence number seq=1.
	if err := c.handshake(s); err != nil {
		_ = conn.Close()
		s.cancel()
		return err
	}

	c.mu.Lock()

	// Discard dial results that arrive while closing, so a destroyed connection is never marked
	// available again and no zombie session is left behind.
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

// handshake performs the handshake synchronously and validates the response.
//
// It does not use any business pending shard.
func (c *ClientConn) handshake(s *session) error {
	const seq = uint64(1)

	req := protocol.EncodeHandshakeReq(seq, c.cli.opts.Kind, c.cli.opts.ID, c.epoch)
	_, err := s.conn.Write(req.Bytes())
	req.Release()
	if err != nil {
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

// push sends a message, releasing buf on failure.
func (c *ClientConn) push(buf *buffer.NocopyBuffer) error {
	if err := c.doPush(buf); err != nil {
		buf.Release()
		return err
	}

	return nil
}

// call sends a request and waits for its response.
func (c *ClientConn) call(ctx context.Context, seq uint64, buf *buffer.NocopyBuffer) (buffer.Buffer, error) {
	call := make(chan *buffer.Bytes, 1)

	c.pending.store(seq, call)

	if err := c.push(buf); err != nil {
		c.pending.delete(seq)
		return nil, err
	}

	if c.cli.opts.CallTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cli.opts.CallTimeout)
		defer cancel()
	}

	select {
	case <-ctx.Done():
		c.discard(seq, call)
		return nil, ctx.Err()
	case res, ok := <-call:
		if !ok {
			return nil, errors.ErrConnectionHanged
		}
		return res, nil
	}
}

// read reads and dispatches incoming messages until the session ends.
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

// write writes queued messages to the connection.
//
// Messages are taken from the queue in batches. An idle connection sends a heartbeat at a fixed
// interval; because a data frame itself proves liveness, no heartbeat is sent while busy. The
// first heartbeat timer includes random jitter to spread out the heartbeat spikes produced when a
// cluster starts all at once.
func (c *ClientConn) write(s *session) {
	var lastWrite time.Time

	timer := time.NewTimer(time.Duration(rand.Int64N(int64(heartbeatInterval))))
	defer timer.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-timer.C:
			// Send a heartbeat only when the connection is idle: a data frame itself proves liveness.
			if time.Since(lastWrite) >= heartbeatInterval {
				if c.cli.opts.WriteTimeout > 0 {
					s.writeDeadline = time.Now().Add(c.cli.opts.WriteTimeout)
					_ = s.conn.SetWriteDeadline(s.writeDeadline)
				}

				if _, err := s.conn.Write(protocol.Heartbeat()); err != nil {
					log.Warnf("write heartbeat message error: %v", err)
					c.retry(s)
					return
				}

				lastWrite = time.Now()
			}

			timer.Reset(heartbeatInterval)
		case buf, ok := <-c.queue.Read():
			if !ok {
				return
			}

			if err := c.doBatchWrite(s, buf); err != nil {
				c.retry(s)
				return
			}

			lastWrite = time.Now()
		}
	}
}

// doBatchWrite writes messages in batches.
//
// It takes tasks from the write queue in batches and sends the collected bytes through a single
// net.Buffers.WriteTo call to reduce the number of system calls.
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
		if timeout := c.cli.opts.WriteTimeout; timeout > 0 {
			now := time.Now()
			// Extend the deadline only when less than half of it remains, avoiding a netpoller
			// syscall on every batch write.
			if now.Add(timeout / 2).After(s.writeDeadline) {
				s.writeDeadline = now.Add(timeout)
				_ = s.conn.SetWriteDeadline(s.writeDeadline)
			}
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

// retry retries the dial after a connection error.
//
// It reconnects only when s is still the current session, so that an old read or write goroutine
// cannot disrupt a new connection.
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

// close closes the connection.
//
// It is called when dial retries are exhausted and moves the connection to the closed state. The
// queue stays open so that a later reconnect can resend messages, but all pending calls are woken
// up so that they do not block until their call timeout.
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

// destroy destroys the connection.
//
// It closes the session and the message queue, releases the backlog of messages and fully
// reclaims the connection's resources.
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

	// Mutual exclusion with queue.Write in doSend prevents a panic from writing to an already
	// closed queue.
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

// wait waits until the connection is reconnected or closed.
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
