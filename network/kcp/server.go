/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/12 12:43 上午
 * @Desc: TODO
 */

package kcp

import (
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/xtaci/kcp-go/v5"
	"net"
	"time"
)

type server struct {
	opts              *serverOptions
	listener          *kcp.Listener
	connMgr           *serverConnMgr
	startHandler      network.StartHandler      // 服务器启动hook函数
	stopHandler       network.CloseHandler      // 服务器关闭hook函数
	connectHandler    network.ConnectHandler    // 连接打开hook函数
	disconnectHandler network.DisconnectHandler // 连接关闭hook函数
	receiveHandler    network.ReceiveHandler    // 接收消息hook函数
}

var _ network.Server = &server{}

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

// Addr 监听地址
func (s *server) Addr() string {
	return s.opts.addr
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

// Protocol 协议
func (s *server) Protocol() string {
	return protocol
}

// OnStart 监听服务器启动
func (s *server) OnStart(handler network.StartHandler) {
	s.startHandler = handler
}

// OnStop 监听服务器关闭
func (s *server) OnStop(handler network.CloseHandler) {
	s.stopHandler = handler
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

// 初始化服务器
func (s *server) init() error {
	//key := pbkdf2.Key([]byte("demo pass"), []byte("demo salt"), 1024, 32, sha1.New)
	//block, _ := kcp.NewAESBlockCrypt(key)

	ln, err := kcp.ListenWithOptions(s.opts.addr, nil, 10, 3)
	if err != nil {
		return err
	}

	s.listener = ln

	return nil
}

// 启动服务器
func (s *server) serve() {
	var tempDelay time.Duration

	for {
		conn, err := s.listener.AcceptKCP()
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}

				log.Warnf("kcp accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}

			log.Warnf("kcp accept error: %v", err)
			return
		}

		tempDelay = 0

		if err = s.connMgr.allocate(conn); err != nil {
			_ = conn.Close()
		}
	}
}
