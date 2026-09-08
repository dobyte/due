package drpc

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/endpoint"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/log"
	taskpool "github.com/dobyte/due/v2/task"
)

const scheme = "drpc"

type RouteHandler func(conn *ServerConn, data []byte) error

type Server struct {
	opts        *ServerOptions     // 配置
	listenAddr  string             // 监听地址
	exposeAddr  string             // 暴露地址
	endpoint    *endpoint.Endpoint // 暴露端点
	started     atomic.Bool        // 是否已启动
	listener    atomic.Value       // 监听器
	handlers    [256]RouteHandler  // 路由处理器
	connections sync.Map           // 连接映射
	replies     *replyCache        // 响应缓存，用于重连后按 seq 幂等去重
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
	s.handlers[route.Handshake] = s.handshake
	s.replies = newReplyCache(replyCacheTTL)

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
	if s.started.Swap(true) {
		return errors.ErrIllegalOperation
	}

	var listener net.Listener

	defer func() {
		if listener == nil {
			s.started.Store(false)
		}
	}()

	addr, err := net.ResolveTCPAddr("tcp", s.listenAddr)
	if err != nil {
		return err
	}

	if listener, err = net.ListenTCP(addr.Network(), addr); err != nil {
		return err
	}

	s.listener.Store(listener)

	go s.serve(listener)

	return nil
}

// Stop 关闭服务器
// @return @1 error 错误信息
func (s *Server) Stop() error {
	if !s.started.Swap(false) {
		return errors.ErrIllegalOperation
	}

	if listener, ok := s.listener.Swap((*net.TCPListener)(nil)).(*net.TCPListener); ok && listener != nil {
		if err := listener.Close(); err != nil {
			log.Warnf("tcp listener close error: %v", err)
		}
	}

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

		cc := conn.(*net.TCPConn)
		cc.SetNoDelay(true)

		s.allocateConn(cc)
	}

	s.listener.Store((*net.TCPListener)(nil))

	s.closeAllConns()
}

// RegisterHandler 注册处理器
func (s *Server) RegisterHandler(route uint8, handler RouteHandler) {
	s.handlers[route] = handler
}

// 处理握手
func (s *Server) handshake(conn *ServerConn, data []byte) error {
	seq, insKind, insID, err := protocol.DecodeHandshakeReq(data)
	if err != nil {
		return err
	}

	if err = conn.doSaveHandshakeInstance(insID, insKind); err != nil {
		return err
	}

	return conn.Send(protocol.EncodeHandshakeRes(seq, codes.OK))
}

// 分配连接
func (s *Server) allocateConn(conn *net.TCPConn) {
	s.connections.Store(conn, newServerConn(s, conn))
}

// 删除连接
func (s *Server) deleteConn(conn *net.TCPConn) {
	s.connections.Delete(conn)
}

// 关闭所有连接
func (s *Server) closeAllConns() error {
	wg, _ := taskpool.WithContext(context.Background())

	s.connections.Range(func(_, conn any) bool {
		wg.Go(func() error {
			return conn.(*ServerConn).Close()
		})

		return true
	})

	return wg.Wait()
}

// replyKey 响应缓存的键
type replyKey struct {
	insID string
	seq   uint64
}

// replyEntry 响应缓存条目
type replyEntry struct {
	data   []byte
	expire time.Time
}

// replyCache 响应缓存，用于重连后按 (insID, seq) 幂等去重
type replyCache struct {
	mu        sync.Mutex
	entries   map[replyKey]*replyEntry
	ttl       time.Duration
	nextSweep time.Time
}

// newReplyCache 创建响应缓存
func newReplyCache(ttl time.Duration) *replyCache {
	return &replyCache{entries: make(map[replyKey]*replyEntry), ttl: ttl}
}

// get 获取缓存的响应，过期或不存在返回 false
func (c *replyCache) get(insID string, seq uint64) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := replyKey{insID: insID, seq: seq}

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expire) {
		delete(c.entries, key)
		return nil, false
	}

	return entry.data, true
}

// store 缓存响应，并周期性地惰性清理过期条目
func (c *replyCache) store(insID string, seq uint64, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	if now.After(c.nextSweep) {
		c.nextSweep = now.Add(c.ttl / 2)

		for key, entry := range c.entries {
			if now.After(entry.expire) {
				delete(c.entries, key)
			}
		}
	}

	c.entries[replyKey{insID: insID, seq: seq}] = &replyEntry{data: data, expire: now.Add(c.ttl)}
}
