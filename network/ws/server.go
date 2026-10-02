package ws

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/gorilla/websocket"
	"github.com/pires/go-proxyproto"
)

// UpgradeHandler is the handler invoked when an HTTP request is upgraded to the WebSocket protocol.
//
// It reports whether the upgrade is allowed.
type UpgradeHandler func(w http.ResponseWriter, r *http.Request) (allowed bool)

// Server is the WebSocket server interface.
type Server interface {
	network.Server
	// OnUpgrade registers the handler invoked when an HTTP request is upgraded.
	OnUpgrade(handler UpgradeHandler)
}

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
	upgradeHandler    UpgradeHandler            // Handler invoked when an HTTP request is upgraded to the WS protocol
}

var _ Server = &server{}

// NewServer returns a new server.
func NewServer(opts ...ServerOption) Server {
	o := defaultServerOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &server{}
	s.opts = o
	s.connMgr = newConnMgr(s)

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

// Protocol returns the protocol name.
func (s *server) Protocol() string {
	return protocol
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

// init initializes the WS server.
//
// It resolves the TCP address and creates a TCP listener; any failure rolls back the start.
func (s *server) init() error {
	if s.listener != nil {
		return errors.ErrServerStarted
	}

	addr, err := net.ResolveTCPAddr("tcp", s.opts.addr)
	if err != nil {
		return err
	}

	ln, err := net.ListenTCP(addr.Network(), addr)
	if err != nil {
		return err
	}

	if s.opts.proxyMode == ProxyModeTransport {
		s.listener = &proxyproto.Listener{Listener: ln}
	} else {
		s.listener = ln
	}

	s.connMgr.open()

	return nil
}

// serve starts the server.
//
// It registers the WebSocket upgrade handler and serves over HTTP or HTTPS according to the
// options. After an upgrade request is validated against the method, the upgrade header and the
// custom upgrade handler, it allocates a connection and closes it when allocation fails.
func (s *server) serve(ln net.Listener) {
	var (
		err      error
		mux      = http.NewServeMux()
		upgrader = websocket.Upgrader{
			ReadBufferSize:    s.opts.readBufferSize,
			WriteBufferSize:   s.opts.writeBufferSize,
			EnableCompression: s.opts.enableCompression,
			CheckOrigin:       s.opts.checkOrigin,
		}
	)

	mux.HandleFunc(s.opts.path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		if !websocket.IsWebSocketUpgrade(r) {
			http.Error(w, http.StatusText(http.StatusUpgradeRequired), http.StatusUpgradeRequired)
			return
		}

		if s.upgradeHandler != nil && !s.upgradeHandler(w, r) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Errorf("websocket upgrade error: %v", err)
			return
		}

		if s.opts.enableCompression {
			conn.EnableWriteCompression(true)
			conn.SetCompressionLevel(s.opts.compressionLevel)
		}

		if err = s.connMgr.allocateConn(conn, s.parseAddrFromHeader(r)); err != nil {
			if errors.Is(err, errors.ErrServerClosed) {
				log.Debugf("connection allocate error: %v", err)
			} else {
				log.Errorf("connection allocate error: %v", err)
			}

			if err = conn.Close(); err != nil {
				log.Errorf("connection close error: %v", err)
			}
		}
	})

	if s.opts.certFile != "" && s.opts.keyFile != "" {
		err = http.ServeTLS(ln, mux, s.opts.certFile, s.opts.keyFile)
	} else {
		err = http.Serve(ln, mux)
	}
	if err != nil && !errors.Is(err, net.ErrClosed) {
		log.Errorf("websocket server shutdown, err: %v", err)
	}

	_ = s.stop(ln)
}

// OnStart registers the handler invoked when the server starts.
func (s *server) OnStart(handler network.StartHandler) {
	s.startHandler = handler
}

// OnStop registers the handler invoked when the server stops.
func (s *server) OnStop(handler network.CloseHandler) {
	s.stopHandler = handler
}

// OnUpgrade registers the handler invoked when an HTTP request is upgraded.
func (s *server) OnUpgrade(handler UpgradeHandler) {
	s.upgradeHandler = handler
}

// OnConnect registers the handler invoked when a connection is opened.
func (s *server) OnConnect(handler network.ConnectHandler) {
	s.connectHandler = handler
}

// OnDisconnect registers the handler invoked when a connection is closed.
func (s *server) OnDisconnect(handler network.DisconnectHandler) {
	s.disconnectHandler = handler
}

// OnHeartbeat registers the handler invoked when a heartbeat is received.
func (s *server) OnHeartbeat(handler network.HeartbeatHandler) {
	s.heartbeatHandler = handler
}

// OnReceive registers the handler invoked when a message is received.
func (s *server) OnReceive(handler network.ReceiveHandler) {
	s.receiveHandler = handler
}

// parseAddrFromHeader parses the real client address from the proxy headers.
//
// It extracts the real client IP and port from the proxy headers only in the application proxy mode
// and when the request comes from a trusted proxy.
func (s *server) parseAddrFromHeader(r *http.Request) net.Addr {
	if s.opts.proxyMode != ProxyModeApplication {
		return nil
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}

	if ip := net.ParseIP(host); ip == nil || !s.isTrustedProxy(ip) {
		return nil
	}

	if addr := s.extractAddrFromHeader(r); addr != nil {
		return addr
	}

	return nil
}

// isTrustedProxy reports whether ip belongs to a trusted proxy.
func (s *server) isTrustedProxy(ip net.IP) bool {
	if ip == nil {
		return false
	}

	if !s.opts.proxyOpts.TrustProxy.Enable {
		return false
	}

	if (s.opts.proxyOpts.TrustProxy.Loopback && ip.IsLoopback()) ||
		(s.opts.proxyOpts.TrustProxy.Private && ip.IsPrivate()) ||
		(s.opts.proxyOpts.TrustProxy.LinkLocal && ip.IsLinkLocalUnicast()) {
		return true
	}

	if len(s.opts.proxyOpts.TrustProxy.ips) > 0 {
		if _, trusted := s.opts.proxyOpts.TrustProxy.ips[ip.String()]; trusted {
			return true
		}
	}

	for _, ipNet := range s.opts.proxyOpts.TrustProxy.ranges {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// extractAddrFromHeader extracts the real client address from the proxy headers.
//
// It walks the IP chain in the proxy header (such as X-Forwarded-For) from right to left, skips
// trusted proxy IPs and returns the first untrusted IP, resolving the client port from the
// X-Forwarded-Port header. It returns nil when the whole chain consists of trusted proxies or
// cannot be parsed.
func (s *server) extractAddrFromHeader(r *http.Request) *net.TCPAddr {
	proxyHeader := "X-Forwarded-For"
	if s.opts.proxyOpts.ProxyHeader != "" {
		proxyHeader = s.opts.proxyOpts.ProxyHeader
	}

	portHeader := "X-Forwarded-Port"
	if s.opts.proxyOpts.PortHeader != "" {
		portHeader = s.opts.proxyOpts.PortHeader
	}

	headerValue := strings.TrimSpace(r.Header.Get(proxyHeader))
	if headerValue == "" {
		return nil
	}

	parts := strings.Split(headerValue, ",")
	ports := strings.Split(strings.TrimSpace(r.Header.Get(portHeader)), ",")

	for i := len(parts) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(parts[i]))
		if ip == nil {
			continue
		}

		if s.isTrustedProxy(ip) {
			continue
		}

		var port int

		if i < len(ports) {
			if p, err := strconv.Atoi(strings.TrimSpace(ports[i])); err == nil && p >= 0 && p <= 65535 {
				port = p
			}
		}

		return &net.TCPAddr{IP: ip, Port: port}
	}

	return nil
}
