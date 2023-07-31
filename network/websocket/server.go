package websocket

import (
	"context"
	"fmt"
	"github.com/cloudwego/netpoll"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/gobwas/ws"
	"net"
	"net/http"
	"time"
)

type UpgradeHandler func(w http.ResponseWriter, r *http.Request) (allowed bool)

type Server interface {
	network.Server
	// OnUpgrade 监听HTTP请求升级
	OnUpgrade(handler UpgradeHandler)
}

type server struct {
	opts              *serverOptions            // 配置
	listener          net.Listener              // 监听器
	connMgr           *connMgr                  // 连接管理器
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
	return "websocket"
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
	upgrader := ws.Upgrader{
		OnBeforeUpgrade: func() (header ws.HandshakeHeader, err error) {
			return nil, nil
		},
	}

	//var tempDelay time.Duration

	eventLoop, _ := netpoll.NewEventLoop(
		func(ctx context.Context, conn netpoll.Connection) error {
			//var reader, _ = conn.Reader(), conn.Writer()

			//buf, _ := io.ReadAll(conn)

			//fmt.Println(reader)

			//fmt.Println(reader.Len())
			//
			//buf, _ := reader.Next(20)
			//
			//fmt.Println(buf)
			//
			//reader.Release()

			c, ok := s.connMgr.load(conn)
			if !ok {
				return errors.New("invalid connection")
			}

			c.read()

			return nil
		},
		netpoll.WithOnConnect(func(ctx context.Context, conn netpoll.Connection) context.Context {
			if _, err := upgrader.Upgrade(conn); err != nil {
				log.Errorf("websocket upgrade error: %v", err)
				_ = conn.Close()
				return nil
			}

			if err := s.connMgr.allocate(conn); err != nil {
				log.Errorf("connection allocate error: %v", err)
				_ = conn.Close()
				return nil
			}

			return ctx
		}),
		netpoll.WithReadTimeout(time.Second),
	)

	err := eventLoop.Serve(s.listener)

	if err != nil {
		fmt.Println(err)
	}

	//for {
	//	conn, err := s.listener.Accept()
	//	if err != nil {
	//		if e, ok := err.(net.Error); ok && e.Temporary() {
	//			if tempDelay == 0 {
	//				tempDelay = 5 * time.Millisecond
	//			} else {
	//				tempDelay *= 2
	//			}
	//			if max := 1 * time.Second; tempDelay > max {
	//				tempDelay = max
	//			}
	//
	//			log.Warnf("tcp accept error: %v; retrying in %v", err, tempDelay)
	//			time.Sleep(tempDelay)
	//			continue
	//		}
	//
	//		log.Errorf("tcp accept error: %v", err)
	//		return
	//	}
	//
	//	_, err = upgrader.Upgrade(conn)
	//	if err != nil {
	//		log.Errorf("websocket upgrade error: %v", err)
	//		continue
	//	}
	//
	//	if err = s.connMgr.allocate(conn); err != nil {
	//		log.Errorf("connection allocate error: %v", err)
	//		_ = conn.Close()
	//	}
	//}
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
