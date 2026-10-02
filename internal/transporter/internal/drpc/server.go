package drpc

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/endpoint"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/log"
	taskpool "github.com/dobyte/due/v2/task"
)

const scheme = "drpc"

type Server struct {
	opts       *ServerOptions     // Options
	listenAddr string             // Listen address
	exposeAddr string             // Exposed address
	endpoint   *endpoint.Endpoint // Exposed endpoint
	mu         sync.Mutex         // Lock
	ctx        context.Context    // Context
	cancel     context.CancelFunc // Cancel function
	listener   *net.TCPListener   // Listener
	handlers   [256]RouteHandler  // Route handlers
	conns      sync.Map           // Connection map
	ticker     *time.Ticker       // Heartbeat ticker
	queues     sync.Map           // Closed queues
	connSeq    atomic.Uint64      // Connection sequence
	workerWg   sync.WaitGroup     // Worker goroutine wait group
}

// NewServer creates a server with the given options.
func NewServer(opts *ServerOptions) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr, opts.Expose)
	if err != nil {
		return nil, err
	}

	s := &Server{}
	s.opts = opts
	s.listenAddr = listenAddr
	s.exposeAddr = exposeAddr
	s.endpoint = endpoint.NewEndpoint(scheme, exposeAddr, false)

	return s, nil
}

// Scheme returns the protocol scheme.
func (s *Server) Scheme() string {
	return scheme
}

// ListenAddr returns the listen address.
func (s *Server) ListenAddr() string {
	return s.listenAddr
}

// ExposeAddr returns the exposed address.
func (s *Server) ExposeAddr() string {
	return s.exposeAddr
}

// Endpoint returns the exposed endpoint.
func (s *Server) Endpoint() *endpoint.Endpoint {
	return s.endpoint
}

// Start starts the server.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.init(); err != nil {
		return err
	}

	go s.serve(s.listener)
	go s.check()

	return nil
}

// Stop stops the server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return errors.ErrServerClosed
	}

	if err := s.listener.Close(); err != nil {
		log.Warnf("tcp listener close error: %v", err)
	}

	s.cancel()
	s.listener = nil
	s.ticker.Stop()
	s.closeAllConns()
	s.clearAllQueues()

	return nil
}

// RegisterHandler registers a handler for the given route.
func (s *Server) RegisterHandler(route uint8, handler RouteHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		log.Warnf("server already started, cannot register handler: %d", route)
		return
	}

	if s.handlers[route] != nil {
		log.Warnf("handler already registered for route: %d", route)
		return
	}

	s.handlers[route] = handler
}

// serve accepts connections.
//
// It accepts TCP connections in a loop and handles each in its own goroutine. Transient errors are
// retried with exponential backoff, and the loop ends when the server is closed.
func (s *Server) serve(listener net.Listener) {
	var delay time.Duration

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}

			if delay == 0 {
				delay = 5 * time.Millisecond
			} else {
				delay *= 2
			}
			if max := 1 * time.Second; delay > max {
				delay = max
			}

			log.Warnf("tcp accept error: %v; retrying in %v", err, delay)
			time.Sleep(delay)
			continue
		}

		delay = 0

		s.allocateConn(conn.(*net.TCPConn))
	}

	_ = s.Stop()
}

// init initializes the server.
func (s *Server) init() error {
	if s.listener != nil {
		return errors.ErrServerStarted
	}

	addr, err := net.ResolveTCPAddr("tcp", s.listenAddr)
	if err != nil {
		return err
	}

	if s.listener, err = net.ListenTCP(addr.Network(), addr); err != nil {
		return err
	}

	s.ticker = time.NewTicker(heartbeatInterval)
	s.ctx, s.cancel = context.WithCancel(context.Background())

	return nil
}

// check checks connections for heartbeat timeouts and cleans up expired queues.
func (s *Server) check() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case t, ok := <-s.ticker.C:
			if !ok {
				return
			}

			// Liveness checks and expired queue cleanup are periodic lightweight traversals, so
			// they run inline in the check goroutine to avoid task pool scheduling overhead.
			s.conns.Range(func(_, cc any) bool {
				cc.(*ServerConn).checkHeartbeat(&t)
				return true
			})

			s.queues.Range(func(k, v any) bool {
				q := v.(*closedQueue)

				if q.time.Add(maxRetentionTime).Before(time.Now()) {
					if s.queues.CompareAndDelete(k, v) {
						for buf := range q.queue.Read() {
							buf.Release()
						}
					}
				}

				return true
			})
		}
	}
}

// handleMessage dispatches a message to the handler registered for its route.
func (s *Server) handleMessage(conn *ServerConn, rt uint8, seq uint64, buf *buffer.Bytes) error {
	if handler := s.handlers[rt]; handler == nil {
		buf.Release()

		return errors.ErrNotFoundRoute
	} else {
		switch rt {
		case route.Push, route.Multicast, route.Broadcast, route.Publish, route.Deliver, route.Trigger:
			return handler(conn, seq, buf)
		default:
			taskpool.Add(func() { handler(conn, seq, buf) })
		}

		return nil
	}
}

// deleteConn removes a connection.
func (s *Server) deleteConn(conn *net.TCPConn) {
	s.conns.Delete(conn)
}

// allocateConn stores a new server connection for conn.
func (s *Server) allocateConn(conn *net.TCPConn) {
	s.conns.Store(conn, newServerConn(s, conn))
}

// closeAllConns force-closes all connections concurrently.
func (s *Server) closeAllConns() {
	wg, _ := taskpool.WithContext(context.Background())

	s.conns.Range(func(_, cc any) bool {
		wg.Go(cc.(*ServerConn).forceClose)
		return true
	})

	wg.Wait()
}

// clearAllQueues drains and removes all cached closed queues.
func (s *Server) clearAllQueues() {
	wg, _ := taskpool.WithContext(context.Background())

	s.queues.Range(func(k, v any) bool {
		if s.queues.CompareAndDelete(k, v) {
			q := v.(*closedQueue)

			wg.Go(func() error {
				for buf := range q.queue.Read() {
					buf.Release()
				}
				return nil
			})
		}

		return true
	})

	wg.Wait()
}

// doLoadQueue loads and removes the cached queue for key, reporting whether it is still valid.
func (s *Server) doLoadQueue(key string) (*queue.Queue[buffer.Buffer], bool) {
	if v, ok := s.queues.LoadAndDelete(key); ok {
		q := v.(*closedQueue)

		if q.time.Add(maxRetentionTime).After(time.Now()) {
			return q.queue, true
		} else {
			for buf := range q.queue.Read() {
				buf.Release()
			}
		}
	}

	return nil, false
}

// doCacheQueue caches a closed queue under key for later replay, draining any previous queue.
func (s *Server) doCacheQueue(key string, queue *queue.Queue[buffer.Buffer]) {
	if v, ok := s.queues.Swap(key, &closedQueue{
		queue: queue,
		time:  time.Now(),
	}); ok {
		q := v.(*closedQueue)

		for buf := range q.queue.Read() {
			buf.Release()
		}
	}
}
