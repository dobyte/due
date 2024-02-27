package gnet

import (
	"context"
	"errors"
	"github.com/panjf2000/gnet/v2"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/network"
	"strings"
	"time"
)

var (
	ErrConnectionReset     = errors.New("connection reset by peer")
	ErrReadConnectionReset = errors.New("read: connection reset by peer")
)

func (s *server) OnBoot(eng gnet.Engine) gnet.Action {
	log.Debugf("running server on %s://%s with multi-core=%t",
		s.Protocol(), s.opts.addr, s.opts.multicore)
	s.eng = eng
	return gnet.None
}

func (s *server) OnOpen(c gnet.Conn) ([]byte, gnet.Action) {
	if err := s.connMgr.allocate(c); err != nil {
		log.Warnf("connection %s allocate error: %v", c.RemoteAddr().String(), err)
		_ = c.Close()
		return nil, gnet.Close
	}

	return nil, gnet.None
}

func (s *server) OnClose(c gnet.Conn, err error) gnet.Action {
	if err != nil && !strings.EqualFold(ErrConnectionReset.Error(), err.Error()) && !strings.EqualFold(ErrReadConnectionReset.Error(), err.Error()) {
		log.Warnf("error occurred on connection=%s, %v\n", c.RemoteAddr().String(), err)
	}

	conn, ok := s.connMgr.load(c)
	if !ok {
		log.Warnf("invalid connection:[%v]", c.RemoteAddr())
		return gnet.Close
	}
	err = conn.forceClose()
	if err != nil {
		log.Warnf("force close connection=%v, uid = %v, encounter error [%v]\n", conn.ID(), conn.UID(), err)
	}

	return gnet.None
}

func (s *server) OnTraffic(c gnet.Conn) (action gnet.Action) {
	conn, ok := s.connMgr.load(c)
	if !ok {
		log.Warnf("invalid connection:[%v]", c.RemoteAddr())
		action = gnet.Close
		return
	}
	for {
		select {
		case <-conn.rBlock:
		inner:
			for {
				select {
				case <-conn.rRelease:
					break inner
				case <-time.After(3 * time.Second):
					log.Warnf("block server read from client timeout")
					_ = conn.Close(true)
					break inner
				}
			}
		default:
			err := conn.read(c)
			if errors.Is(err, ErrIncompletePacket) {
				return
			}
			if err != nil {
				log.Warnf("connection:[%v] uid:[%v], connection read encounter error : [%v]", conn.ID(), conn.UID(), err)
				err = conn.forceClose()
				if err != nil {
					log.Warnf("force close connection=%v, uid = %v, encounter error [%v]\n", conn.ID(), conn.UID(), err)
				}
				action = gnet.Close
				return
			}
		}
	}

	return
}

type server struct {
	gnet.BuiltinEventEngine
	eng               gnet.Engine
	opts              *serverOptions            // 配置
	connMgr           *serverConnMgr            // 连接管理器
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
	s.connMgr = newConnMgr(s)

	return s
}

// Addr 监听地址
func (s *server) Addr() string {
	return s.opts.addr
}

// Protocol 协议
func (s *server) Protocol() string {
	return "tcp"
}

// Start 启动服务器
func (s *server) Start() error {
	if err := s.init(); err != nil {
		return err
	}

	if s.startHandler != nil {
		s.startHandler()
	}

	return nil
}

// Stop 关闭服务器
func (s *server) Stop() (err error) {
	err = s.eng.Stop(context.Background())

	s.connMgr.close()

	return
}

func (s *server) init() error {
	go func() {
		err := gnet.Run(s, s.Protocol()+"://"+s.opts.addr, gnet.WithMulticore(s.opts.multicore), gnet.WithReuseAddr(true), gnet.WithReusePort(true))
		if err != nil {
			s.eng.Stop(context.Background())
			log.Fatalf("init gate gnet server failed, %v", err)
		}
	}()

	return nil
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
