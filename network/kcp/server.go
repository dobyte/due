package kcp

import (
	"net"
	"sync"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/pires/go-proxyproto"
	"github.com/xtaci/kcp-go/v5"
)

type server struct {
	opts              *serverOptions            // Options
	mu                sync.Mutex                // Lock
	listener          net.Listener              // Listener
	connMgr           *serverConnMgr            // Connection manager
	startHandler      network.StartHandler      // Hook invoked when the server starts
	stopHandler       network.CloseHandler      // Hook invoked when the server stops
	connectHandler    network.ConnectHandler    // Hook invoked when a connection is opened
	disconnectHandler network.DisconnectHandler // Hook invoked when a connection is closed
	heartbeatHandler  network.HeartbeatHandler  // Hook invoked on connection heartbeat
	receiveHandler    network.ReceiveHandler    // Hook invoked when a message is received
}

var _ network.Server = &server{}

// NewServer returns a new server.
//
// The default options are overridden by the given opts, after which the connection manager is
// initialized.
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
// Once the server has started it returns the actual address of the listener; before that it
// returns the configured address.
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
//
// It initializes the listener, serves connections in a goroutine and then invokes the start hook.
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
//
// It closes the listener and every connection, and then invokes the stop hook.
func (s *server) Stop() error {
	return s.stop(nil)
}

// stop stops the server.
//
// It closes the listener and all connections. When ln is non-nil it is closed only while it is
// still the current listener, so that an old serve goroutine exiting does not accidentally close a
// new listener created by a restart.
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

// Protocol returns the protocol name "kcp".
func (s *server) Protocol() string {
	return protocol
}

// OnStart registers handler to be invoked when the server starts.
func (s *server) OnStart(handler network.StartHandler) {
	s.startHandler = handler
}

// OnStop registers handler to be invoked when the server stops.
func (s *server) OnStop(handler network.CloseHandler) {
	s.stopHandler = handler
}

// OnConnect registers handler to be invoked when a connection is opened.
func (s *server) OnConnect(handler network.ConnectHandler) {
	s.connectHandler = handler
}

// OnDisconnect registers handler to be invoked when a connection is closed.
func (s *server) OnDisconnect(handler network.DisconnectHandler) {
	s.disconnectHandler = handler
}

// OnHeartbeat registers handler to be invoked on a connection heartbeat.
func (s *server) OnHeartbeat(handler network.HeartbeatHandler) {
	s.heartbeatHandler = handler
}

// OnReceive registers handler to be invoked when a message is received.
func (s *server) OnReceive(handler network.ReceiveHandler) {
	s.receiveHandler = handler
}

// init initializes the server.
//
// It creates the KCP listener and marks the server as started.
func (s *server) init() error {
	if s.listener != nil {
		return errors.ErrServerStarted
	}

	addr, err := net.ResolveTCPAddr("tcp", s.opts.addr)
	if err != nil {
		return err
	}

	if s.listener, err = kcp.ListenWithOptions(addr.String(), nil, 0, 0); err != nil {
		return err
	}

	if s.opts.enableProxyProtocol {
		s.listener = &proxyproto.Listener{Listener: s.listener}
	}

	s.connMgr.open()

	return nil
}

// serve accepts KCP connections in a loop and hands them to the connection manager.
//
// Temporary errors, such as running out of file descriptors, are retried with exponential backoff
// so that a transient hiccup does not bring the server down. When listening ends, every connection
// is closed.
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

				log.Warnf("kcp accept temporary error: %v, retrying in %v", err, tempDelay)

				time.Sleep(tempDelay)

				continue
			}

			log.Warnf("kcp accept error: %v", err)
			break
		}

		tempDelay = 0

		var session *kcp.UDPSession
		if pc, ok := conn.(*proxyproto.Conn); ok {
			session = pc.Raw().(*kcp.UDPSession)
		} else {
			session = conn.(*kcp.UDPSession)
		}

		if err = s.connMgr.allocateConn(session); err != nil {
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
