package quic

import (
	"context"
	"crypto/tls"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
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

// NewServer creates a QUIC server. Register handlers before starting it.
func NewServer(opts ...ServerOption) network.Server {
	o := defaultServerOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &server{opts: o}
}

// Addr returns the bound address while running, or the configured address otherwise.
func (s *server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.run != nil {
		return s.run.listener.Addr().String()
	}
	return s.opts.addr
}

// Start starts a fresh server lifecycle, including after a completed Stop.
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

// Stop stops accepting connections and closes pending and active transports.
// Application callbacks may finish after Stop returns, so callbacks can call Stop.
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
			break
		}
		id, ok := r.manager.reserve(qc)
		if !ok {
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
		_ = qc.CloseWithError(0, "server stopped")
		r.manager.remove(id)
		return
	}
}

// Protocol returns the protocol name.
func (s *server) Protocol() string { return protocol }

// OnStart registers the start handler.
func (s *server) OnStart(h network.StartHandler) { s.startHandler = h }

// OnStop registers the stop handler.
func (s *server) OnStop(h network.CloseHandler) { s.stopHandler = h }

// OnConnect registers the connection handler.
func (s *server) OnConnect(h network.ConnectHandler) { s.connectHandler = h }

// OnDisconnect registers the disconnection handler.
func (s *server) OnDisconnect(h network.DisconnectHandler) { s.disconnectHandler = h }

// OnReceive registers the message handler, which owns each received buffer.
func (s *server) OnReceive(h network.ReceiveHandler) { s.receiveHandler = h }

// OnHeartbeat registers the heartbeat handler.
func (s *server) OnHeartbeat(h network.HeartbeatHandler) { s.heartbeatHandler = h }
