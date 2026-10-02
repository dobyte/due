package tcp

import (
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/pires/go-proxyproto"
)

type server struct {
	opts              *serverOptions            // Options
	mu                sync.Mutex                // Lock
	listener          net.Listener              // Listener
	connMgr           *serverConnMgr            // Connection manager
	startHandler      network.StartHandler      // Handler invoked when the server starts
	stopHandler       network.CloseHandler      // Handler invoked when the server stops
	connectHandler    network.ConnectHandler    // Handler invoked when a connection is opened
	disconnectHandler network.DisconnectHandler // Handler invoked when a connection is closed
	heartbeatHandler  network.HeartbeatHandler  // Handler invoked when a heartbeat is received
	receiveHandler    network.ReceiveHandler    // Handler invoked when a message is received
}

var _ network.Server = &server{}

// NewServer returns a new server.
func NewServer(opts ...ServerOption) network.Server {
	o := defaultServerOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &server{}
	s.opts = o
	s.connMgr = newServerConnMgr(s)

	return s
}

// Addr returns the listen address.
//
// It returns the actual listener address after the server starts and the configured address before
// that.
func (s *server) Addr() string {
	s.mu.Lock()

	if s.listener != nil {
		addr := s.listener.Addr().String()
		s.mu.Unlock()
		return addr
	}

	addr := s.opts.addr
	s.mu.Unlock()

	return addr
}

// Start starts the server.
func (s *server) Start() error {
	s.mu.Lock()

	if err := s.init(); err != nil {
		s.mu.Unlock()
		return err
	}

	ln := s.listener

	go s.serve(ln)

	s.mu.Unlock()

	if s.startHandler != nil {
		s.startHandler()
	}

	return nil
}

// Stop stops the server.
func (s *server) Stop() error {
	return s.stop(nil)
}

// stop stops the server.
//
// It closes the listener and closes all connections. When ln is not nil the close is performed only
// if it is still the current listener, which prevents an old serve goroutine from closing the new
// listener after a restart by mistake.
func (s *server) stop(ln net.Listener) error {
	s.mu.Lock()

	if s.listener == nil || (ln != nil && s.listener != ln) {
		s.mu.Unlock()
		return errors.ErrServerClosed
	}

	_ = s.listener.Close()
	s.listener = nil
	s.mu.Unlock()

	s.connMgr.close()

	if s.stopHandler != nil {
		s.stopHandler()
	}

	return nil
}

// Protocol returns the protocol name.
func (s *server) Protocol() string {
	return protocol
}

// OnStart registers the handler invoked when the server starts.
//
// It must be registered before Start; registering it after Start causes a data race.
func (s *server) OnStart(handler network.StartHandler) {
	s.startHandler = handler
}

// OnStop registers the handler invoked when the server stops.
//
// It must be registered before Start; registering it after Start causes a data race.
func (s *server) OnStop(handler network.CloseHandler) {
	s.stopHandler = handler
}

// OnConnect registers the handler invoked when a connection is opened.
//
// It must be registered before Start; registering it after Start causes a data race.
func (s *server) OnConnect(handler network.ConnectHandler) {
	s.connectHandler = handler
}

// OnDisconnect registers the handler invoked when a connection is closed.
//
// It must be registered before Start; registering it after Start causes a data race.
func (s *server) OnDisconnect(handler network.DisconnectHandler) {
	s.disconnectHandler = handler
}

// OnHeartbeat registers the handler invoked when a heartbeat is received.
//
// It must be registered before Start; registering it after Start causes a data race.
func (s *server) OnHeartbeat(handler network.HeartbeatHandler) {
	s.heartbeatHandler = handler
}

// OnReceive registers the handler invoked when a message is received.
//
// It must be registered before Start; registering it after Start causes a data race.
func (s *server) OnReceive(handler network.ReceiveHandler) {
	s.receiveHandler = handler
}

// init initializes the TCP server.
//
// It resolves the TCP address and creates a TLS or plain TCP listener according to the options; any
// failure rolls back the start.
func (s *server) init() error {
	if s.listener != nil {
		return errors.ErrServerStarted
	}

	addr, err := net.ResolveTCPAddr("tcp", s.opts.addr)
	if err != nil {
		return err
	}

	if s.opts.certFile != "" && s.opts.keyFile != "" {
		cert, err := tls.LoadX509KeyPair(s.opts.certFile, s.opts.keyFile)
		if err != nil {
			return err
		}

		if s.listener, err = tls.Listen(addr.Network(), addr.String(), &tls.Config{
			Certificates: []tls.Certificate{cert},
		}); err != nil {
			return err
		}
	} else {
		if s.listener, err = net.ListenTCP(addr.Network(), addr); err != nil {
			return err
		}
	}

	if s.opts.enableProxyProtocol {
		s.listener = &proxyproto.Listener{Listener: s.listener}
	}

	s.connMgr.open()

	return nil
}

// serve waits for connections.
//
// It accepts TCP connections in a loop and dispatches each one to its own goroutine. Temporary
// errors such as file descriptor exhaustion are retried with exponential backoff so that a
// transient hiccup does not make the server exit; it ends when the server is closed or an
// unrecoverable error occurs.
func (s *server) serve(ln net.Listener) {
	var tempDelay time.Duration

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}

			var ne net.Error
			if errors.As(err, &ne) && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}

				if maxDelay := time.Second; tempDelay > maxDelay {
					tempDelay = maxDelay
				}

				log.Warnf("tcp accept temporary error: %v, retrying in %v", err, tempDelay)

				time.Sleep(tempDelay)

				continue
			}

			log.Warnf("tcp accept error: %v", err)
			break
		}

		tempDelay = 0

		setNoDelay(conn)

		if err = s.connMgr.allocateConn(conn); err != nil {
			if errors.Is(err, errors.ErrServerClosed) {
				log.Debugf("connection allocate error: %v", err)
			} else {
				log.Errorf("connection allocate error: %v", err)
			}
			_ = conn.Close()
		}
	}

	_ = s.stop(ln)
}
