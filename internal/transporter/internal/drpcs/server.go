package drpcs

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/endpoint"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
)

type RouteHandler func(conn *ServerConn, seq uint64, buf buffer.Buffer) error

const scheme = "drpc"

type Server struct {
	opts       *ServerOptions     // 配置
	listenAddr string             // 监听地址
	exposeAddr string             // 暴露地址
	endpoint   *endpoint.Endpoint // 暴露端点
	mu         sync.Mutex         // 锁
	started    bool               // 是否已启动
	listener   *net.TCPListener   // 监听器
	handlers   [256]RouteHandler  // 路由处理器
	conns      sync.Map           // 连接映射
	ticker     *time.Ticker       // 心跳定时器
	workers    []*ServerWorker    // 工作协程
}

// NewServer 创建一个服务器
// @param opts ...ServerOption 服务器配置项
// @return @1 network.Server 服务器实例
func NewServer(opts *ServerOptions) (*Server, error) {
	listenAddr, exposeAddr, err := xnet.ParseAddr(opts.Addr, opts.Expose)
	if err != nil {
		return nil, err
	}

	s := &Server{}
	s.opts = opts
	s.listenAddr = listenAddr
	s.exposeAddr = exposeAddr
	s.endpoint = endpoint.NewEndpoint(scheme, exposeAddr, false)
	s.ticker = time.NewTicker(defaultHeartbeatInterval)

	return s, nil
}

// Scheme 协议
func (s *Server) Scheme() string {
	return scheme
}

// ListenAddr 监听地址
func (s *Server) ListenAddr() string {
	return s.listenAddr
}

// ExposeAddr 暴露地址
func (s *Server) ExposeAddr() string {
	return s.exposeAddr
}

// Endpoint 暴露端点
func (s *Server) Endpoint() *endpoint.Endpoint {
	return s.endpoint
}

// Start 启动服务器
// @return @1 error 错误信息
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return errors.ErrServerStarted
	}

	addr, err := net.ResolveTCPAddr("tcp", s.listenAddr)
	if err != nil {
		return err
	}

	listener, err := net.ListenTCP(addr.Network(), addr)
	if err != nil {
		return err
	}

	s.started = true
	s.listener = listener

	go s.serve(listener)
	go s.check()

	return nil
}

// Stop 关闭服务器
// @return @1 error 错误信息
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return errors.ErrServerClosed
	}

	if err := s.listener.Close(); err != nil {
		log.Warnf("tcp listener close error: %v", err)
	}

	s.started = false
	s.listener = nil
	s.ticker.Stop()
	s.closeAllConns()

	return nil
}

// serve 等待连接
// 循环接受TCP连接并分配到独立协程处理；对瞬时错误采用指数退避重试，服务器关闭时结束
func (s *Server) serve(listener net.Listener) {
	var delay time.Duration

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}

			if delay == 0 {
				delay = 5 * time.Millisecond
			} else {
				delay *= 2
			}
			if max := 1 * time.Second; delay > max {
				delay = max
			}

			log.Warnf("tcp accept error: %v; retrying in %v", err, delay)
			time.Sleep(delay)
			continue
		}

		delay = 0

		s.allocateConn(conn.(*net.TCPConn))
	}

	_ = s.Stop()
}

// check 检查连接是否超时
func (s *Server) check() {
	for t := range s.ticker.C {
		s.conns.Range(func(_, cc any) bool {
			conn := cc.(*ServerConn)

			task.Add(func() {
				if !conn.checkHeartbeat(&t) {
					conn.graceClose()
				}
			})

			return true
		})
	}
}

// 删除连接
func (s *Server) deleteConn(conn *net.TCPConn) {
	s.conns.Delete(conn)
}

// 分配连接
func (s *Server) allocateConn(conn *net.TCPConn) {
	s.conns.Store(conn, newServerConn(s, conn))
}

// 关闭所有连接
func (s *Server) closeAllConns() error {
	wg, _ := task.WithContext(context.Background())

	s.conns.Range(func(_, cc any) bool {
		wg.Go(cc.(*ServerConn).forceClose)
		return true
	})

	return wg.Wait()
}

func (s *Server) handshakeHandler(conn *ServerConn, seq uint64, buf buffer.Buffer) error {
	protocol.DecodeHandshakeReq()

	return nil
}

func (s *Server) messageHandler(conn *ServerConn, route uint8, seq uint64, buf buffer.Buffer) error {
	return nil
}
