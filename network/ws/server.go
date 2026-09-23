/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/3/29 7:45 下午
 * @Desc: Websocket服务器
 */

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

type UpgradeHandler func(w http.ResponseWriter, r *http.Request) (allowed bool)

type Server interface {
	network.Server
	// OnUpgrade 监听HTTP请求升级
	OnUpgrade(handler UpgradeHandler)
}

type server struct {
	opts              *serverOptions            // 配置
	mu                sync.Mutex                // 锁
	listener          net.Listener              // 监听器
	connMgr           *serverConnMgr            // 连接管理器
	startHandler      network.StartHandler      // 服务器启动hook函数
	stopHandler       network.CloseHandler      // 服务器关闭hook函数
	connectHandler    network.ConnectHandler    // 连接打开hook函数
	disconnectHandler network.DisconnectHandler // 连接关闭hook函数
	heartbeatHandler  network.HeartbeatHandler  // 连接心跳hook函数
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
	upgradeHandler    UpgradeHandler            // HTTP协议升级成WS协议hook函数
}

var _ Server = &server{}

// NewServer 创建一个服务器
// @param opts ...ServerOption 服务器配置项
// @return @1 Server 服务器实例
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

// Addr 获取监听地址
// @return @1 string 监听地址
func (s *server) Addr() string {
	return s.opts.addr
}

// Protocol 获取协议名称
// @return @1 string 协议名称
func (s *server) Protocol() string {
	return protocol
}

// Start 启动服务器
// @return @1 error 错误信息
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

// Stop 关闭服务器
// @return @1 error 错误信息
func (s *server) Stop() error {
	s.mu.Lock()
	if s.listener != nil {
		s.listener.Close()
		s.listener = nil
	} else {
		s.mu.Unlock()
		return errors.ErrServerClosed
	}
	s.mu.Unlock()

	s.connMgr.close()

	if s.stopHandler != nil {
		s.stopHandler()
	}

	return nil
}

// init 初始化WS服务器
// 解析TCP地址并创建TCP监听器；若任一环节失败则回滚启动状态
// @return @1 error 已启动或监听地址不合法时返回的错误
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

	return nil
}

// serve 启动服务器
// 注册Websocket升级处理器，按配置以HTTP或HTTPS方式启动服务：
// 升级请求校验方法/升级头/自定义升级钩子后，分配连接对象，失败则关闭连接
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
			log.Errorf("connection allocate error: %v", err)

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
	if err != nil {
		log.Errorf("websocket server shutdown, err: %v", err)
	}

	_ = s.Stop()
}

// OnStart 监听服务器启动
// @param handler network.StartHandler 服务器启动处理函数
func (s *server) OnStart(handler network.StartHandler) {
	s.startHandler = handler
}

// OnStop 监听服务器关闭
// @param handler network.CloseHandler 服务器关闭处理函数
func (s *server) OnStop(handler network.CloseHandler) {
	s.stopHandler = handler
}

// OnUpgrade 监听HTTP请求升级
// @param handler UpgradeHandler HTTP请求升级处理函数
func (s *server) OnUpgrade(handler UpgradeHandler) {
	s.upgradeHandler = handler
}

// OnConnect 监听连接打开
// @param handler network.ConnectHandler 连接打开处理函数
func (s *server) OnConnect(handler network.ConnectHandler) {
	s.connectHandler = handler
}

// OnDisconnect 监听连接关闭
// @param handler network.DisconnectHandler 连接关闭处理函数
func (s *server) OnDisconnect(handler network.DisconnectHandler) {
	s.disconnectHandler = handler
}

// OnHeartbeat 监听心跳
// @param handler network.HeartbeatHandler 心跳处理函数
func (s *server) OnHeartbeat(handler network.HeartbeatHandler) {
	s.heartbeatHandler = handler
}

// OnReceive 监听接收到消息
// @param handler network.ReceiveHandler 消息接收处理函数
func (s *server) OnReceive(handler network.ReceiveHandler) {
	s.receiveHandler = handler
}

// parseAddrFromHeader 从代理头解析客户端真实地址
// 仅在应用层代理模式下，且请求来自受信任代理时，从代理头中提取客户端真实IP及端口。
// @param r *http.Request HTTP请求
// @return @1 net.Addr 客户端真实地址，无法解析时返回nil
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

// isTrustedProxy 是否受信任的代理
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

// extractAddrFromHeader 从代理头中提取客户端真实地址
// 从右向左遍历代理头中的 IP 链（如 X-Forwarded-For），跳过受信任代理 IP，
// 返回第一个非受信任 IP，并从 X-Forwarded-Port 头解析客户端端口；
// 若整条链均为受信任代理或无法解析，则返回 nil。
// @param r *http.Request HTTP请求
// @return @1 *net.TCPAddr 客户端真实地址，无法确定时返回 nil
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
