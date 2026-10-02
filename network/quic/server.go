package quic

import (
	"context"
	"crypto/tls"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/quic-go/quic-go"
)

type serverRun struct {
	ctx      context.Context
	cancel   context.CancelFunc
	listener *quic.Listener
	manager  *serverConnMgr
	done     chan struct{}
	stopping bool
}

type server struct {
	opts              *serverOptions
	id                atomic.Int64
	mu                sync.Mutex
	run               *serverRun
	startHandler      network.StartHandler
	stopHandler       network.CloseHandler
	connectHandler    network.ConnectHandler
	disconnectHandler network.DisconnectHandler
	receiveHandler    network.ReceiveHandler
	heartbeatHandler  network.HeartbeatHandler
}

var _ network.Server = (*server)(nil)

// NewServer returns a new QUIC server.
//
// Every hook must be registered before starting.
func NewServer(opts ...ServerOption) network.Server {
	o := defaultServerOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &server{opts: o}
}

// Addr returns the listen address.
//
// Once the server has started it returns the actual address of the listener; before that it
// returns the configured address.
func (s *server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run != nil {
		return s.run.listener.Addr().String()
	}
	return s.opts.addr
}

// Start starts the server.
//
// Each call opens a fresh server lifecycle, so the server can be started again after Stop has
// completed.
func (s *server) Start() error {
	s.mu.Lock()
	if s.run != nil {
		s.mu.Unlock()
		return errors.ErrIllegalOperation
	}
	cert, err := tls.LoadX509KeyPair(s.opts.certFile, s.opts.keyFile)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	config := transportConfig(s.opts.heartbeatInterval)
	config.HandshakeIdleTimeout = s.opts.handshakeTimeout
	ln, err := quic.ListenAddr(s.opts.addr, &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{alpn}}, config)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &serverRun{ctx: ctx, cancel: cancel, listener: ln, manager: newServerConnMgr(s), done: make(chan struct{})}
	s.run = r
	s.mu.Unlock()
	go s.serve(r)
	if s.startHandler != nil {
		s.startHandler()
	}
	return nil
}

// Stop stops the server.
//
// It stops accepting new connections and closes pending as well as active transport connections.
// Application-level callbacks may still be running after Stop returns, so Stop may also be called
// from within a callback.
func (s *server) Stop() error {
	s.mu.Lock()
	r := s.run
	if r == nil || r.stopping {
		s.mu.Unlock()
		return errors.ErrIllegalOperation
	}
	r.stopping = true
	r.cancel()
	s.mu.Unlock()
	err := r.listener.Close()
	r.manager.close()
	<-r.done
	return err
}

func (s *server) serve(r *serverRun) {
	var pending sync.WaitGroup
	for {
		qc, err := r.listener.Accept(r.ctx)
		if err != nil {
			if r.ctx.Err() == nil {
				log.Warnf("quic accept error: %v", err)
			}
			break
		}
		id, ok := r.manager.reserve(qc)
		if !ok {
			if r.manager.closed.Load() {
				log.Debugf("connection reserve failed: server is closing")
			} else {
				log.Errorf("connection reserve failed: connection limit reached")
			}
			_ = qc.CloseWithError(0, "connection limit reached")
			continue
		}
		pending.Add(1)
		go func() { defer pending.Done(); s.handleConn(r, id, qc) }()
	}
	r.cancel()
	_ = r.listener.Close()
	r.manager.close()
	pending.Wait()
	s.mu.Lock()
	if s.run == r {
		s.run = nil
	}
	close(r.done)
	s.mu.Unlock()
	if s.stopHandler != nil {
		s.stopHandler()
	}
}

func (s *server) handleConn(r *serverRun, id int64, qc *quic.Conn) {
	ctx, cancel := context.WithTimeout(r.ctx, s.opts.handshakeTimeout)
	defer cancel()
	stream, err := qc.AcceptStream(ctx)
	if err != nil {
		_ = qc.CloseWithError(0, "stream accept failed")
		r.manager.remove(id)
		return
	}
	if r.manager.allocateConn(id, qc, stream) == nil {
		log.Debugf("connection allocate failed: server is closing")
		_ = qc.CloseWithError(0, "server stopped")
		r.manager.remove(id)
		return
	}
}

// Protocol returns the protocol name.
func (s *server) Protocol() string { return protocol }

// OnStart registers h to be invoked when the server starts.
//
// It must be registered before Start; registering it afterwards races with Start.
func (s *server) OnStart(h network.StartHandler) { s.startHandler = h }

// OnStop registers h to be invoked when the server stops.
//
// It must be registered before Start; registering it afterwards races with Start.
func (s *server) OnStop(h network.CloseHandler) { s.stopHandler = h }

// OnConnect registers h to be invoked when a connection is opened.
//
// It must be registered before Start; registering it afterwards races with Start.
func (s *server) OnConnect(h network.ConnectHandler) { s.connectHandler = h }

// OnDisconnect registers h to be invoked when a connection is closed.
//
// It must be registered before Start; registering it afterwards races with Start.
func (s *server) OnDisconnect(h network.DisconnectHandler) { s.disconnectHandler = h }

// OnReceive registers h to be invoked when a message is received.
//
// It must be registered before Start; registering it afterwards races with Start. The handler owns
// every received buffer.
func (s *server) OnReceive(h network.ReceiveHandler) { s.receiveHandler = h }

// OnHeartbeat registers h to be invoked on a connection heartbeat.
//
// It must be registered before Start; registering it afterwards races with Start.
func (s *server) OnHeartbeat(h network.HeartbeatHandler) { s.heartbeatHandler = h }
