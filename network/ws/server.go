/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/3/29 7:45 下午
 * @Desc: Websocket服务器
 */

package ws

import (
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
)

const protocol = "websocket"

type UpgradeHandler func(w http.ResponseWriter, r *http.Request) (allowed bool)

type Server interface {
	network.Server
	// OnUpgrade 监听HTTP请求升级
	OnUpgrade(handler UpgradeHandler)
}

type server struct {
	opts              *serverOptions            // 配置
	listener          net.Listener              // 监听器
	connMgr           *serverConnMgr            // 连接管理器
	startHandler      network.StartHandler      // 服务器启动hook函数
	stopHandler       network.CloseHandler      // 服务器关闭hook函数
	connectHandler    network.ConnectHandler    // 连接打开hook函数
	disconnectHandler network.DisconnectHandler // 连接关闭hook函数
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
	upgradeHandler    UpgradeHandler            // HTTP协议升级成WS协议hook函数
}

var _ Server = &server{}

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

// Addr 监听地址
func (s *server) Addr() string {
	return s.opts.addr
}

// Protocol 协议
func (s *server) Protocol() string {
	return protocol
}

// Start 启动服务器
func (s *server) Start() error {
	if err := s.init(); err != nil {
		return err
	}

	if s.startHandler != nil {
		s.startHandler()
	}

	go s.serve()

	return nil
}

// Stop 关闭服务器
func (s *server) Stop() error {
	if err := s.listener.Close(); err != nil {
		return err
	}

	s.connMgr.close()

	return nil
}

// 初始化服务器
func (s *server) init() error {
	addr, err := net.ResolveTCPAddr("tcp", s.opts.addr)
	if err != nil {
		return err
	}

	ln, err := net.ListenTCP(addr.Network(), addr)
	if err != nil {
		return err
	}

	s.listener = ln

	return nil
}

// 启动服务器
func (s *server) serve() {
	upgrader := websocket.Upgrader{
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		EnableCompression: true,
		CheckOrigin:       s.opts.checkOrigin,
	}

	http.HandleFunc(s.opts.path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
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

		if err = s.connMgr.allocate(conn); err != nil {
			log.Errorf("connection allocate error: %v", err)
			_ = conn.Close()
		}
	})

	var err error
	if s.opts.certFile != "" && s.opts.keyFile != "" {
		err = http.ServeTLS(s.listener, nil, s.opts.certFile, s.opts.keyFile)
	} else {
		err = http.Serve(s.listener, nil)
	}

	if err != nil {
		log.Errorf("websocket server shutdown, err: %v", err)
	}
}

// OnStart 监听服务器启动
func (s *server) OnStart(handler network.StartHandler) {
	s.startHandler = handler
}

// OnStop 监听服务器关闭
func (s *server) OnStop(handler network.CloseHandler) {
	s.stopHandler = handler
}

// OnUpgrade 监听HTTP请求升级
func (s *server) OnUpgrade(handler UpgradeHandler) {
	s.upgradeHandler = handler
}

// OnConnect 监听连接打开
func (s *server) OnConnect(handler network.ConnectHandler) {
	s.connectHandler = handler
}

// OnDisconnect 监听连接关闭
func (s *server) OnDisconnect(handler network.DisconnectHandler) {
	s.disconnectHandler = handler
}

// OnReceive 监听接收到消息
func (s *server) OnReceive(handler network.ReceiveHandler) {
	s.receiveHandler = handler
}
