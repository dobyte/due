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
	svr               *Server                     // Server
	rw                sync.RWMutex                // Lock
	ctx               context.Context             // Context
	cancel            context.CancelFunc          // Cancel function
	wg1               *sync.WaitGroup             // Read wait group
	wg2               *sync.WaitGroup             // Write wait group
	conn              *net.TCPConn                // Underlying connection
	state             atomic.Int32                // Connection state
	queue             *queue.Queue[buffer.Buffer] // Message queue
	dueBuffers        []buffer.Buffer             // Message buffers pending write
	netBuffers        net.Buffers                 // Byte slices pending write
	writeDeadline     time.Time                   // Write deadline (owned by the write goroutine)
	lastHeartbeatTime atomic.Int64                // Time of the last heartbeat
	key               string                      // Connection key
	kind              cluster.Kind                // Instance kind
	inst              string                      // Instance ID
	epoch             uint64                      // Connection epoch
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

// Push pushes a message to the connection.
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

// HandshakeInfo returns the instance kind and instance ID negotiated during the handshake.
func (s *ServerConn) HandshakeInfo() (cluster.Kind, string) {
	return s.kind, s.inst
}

// read reads messages from the stream.
//
// It continuously reads messages, refreshing the liveness time on every data frame (a data frame
// acts as a heartbeat), detecting empty and heartbeat packets, and dispatching the rest to the
// receive hook. A read error triggers a forced close.
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
				// Refresh the liveness time on every data frame, so liveness stays accurate even
				// after the client suppresses idle heartbeats.
				c.lastHeartbeatTime.Store(time.Now().UnixNano())

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

					c.svr.handleMessage(c, rt, seq, buf)
				}
			}
		}
	}
}

// write writes queued messages to the connection.
//
// It also checks for heartbeat timeouts at a fixed interval and triggers a forced close on
// timeout.
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

// doBatchWrite writes messages in batches.
//
// It takes tasks from the write queue in batches and sends the collected bytes through a single
// net.Buffers.WriteTo call to reduce the number of system calls.
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
		if timeout := c.svr.opts.WriteTimeout; timeout > 0 {
			now := time.Now()
			// Extend the deadline only when less than half of it remains, avoiding a netpoller
			// syscall on every batch write.
			if now.Add(timeout / 2).After(c.writeDeadline) {
				c.writeDeadline = now.Add(timeout)
				_ = conn.SetWriteDeadline(c.writeDeadline)
			}
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

// checkHeartbeat force-closes the connection when its heartbeat has timed out at time t.
func (c *ServerConn) checkHeartbeat(t *time.Time) {
	if c.lastHeartbeatTime.Load() < t.Add(-2*heartbeatInterval).UnixNano() {
		taskpool.Add(func() { c.forceClose() })
	}
}

// checkState returns an error matching the connection state.
//
// It reports [errors.ErrConnectionNotAlived] for a connection that has not completed the
// handshake, [errors.ErrConnectionHanged] for a hung connection, [errors.ErrConnectionClosed] for
// a closed connection and nil when the connection is alive.
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

// graceClose closes the connection gracefully.
//
// It writes a close signal and waits for the write queue to drain before closing the connection,
// so that the buffered messages are delivered as far as possible.
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

// forceClose closes the connection immediately.
//
// It switches the state to closed and closes the connection without waiting for the write queue to
// drain. It reports [errors.ErrConnectionClosed] when the connection is already closed.
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

	// The queue is not closed yet, so the write goroutine can only exit through ctx and no longer
	// drains the remaining messages.
	c.wg2.Wait()

	// Close the queue so that a replaying range loop terminates after reading the remaining
	// messages.
	c.queue.Close()

	if key != "" {
		c.svr.doCacheQueue(key, c.queue)
	}

	err := conn.Close()

	c.wg1.Wait()
	c.svr.deleteConn(conn)

	return err
}

// doHandshake handles a handshake request.
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
