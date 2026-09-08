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

type RouteHandler func(conn *ServerConn, seq uint64, data []byte) error

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
func (s *Server) handshake(conn *ServerConn, seq uint64, data []byte) error {
	_, epoch, insKind, insID, err := protocol.DecodeHandshakeReq(data)
	if err != nil {
		return err
	}

	if err = conn.doSaveHandshakeInstance(insID, insKind, epoch); err != nil {
		return err
	}

	return conn.Reply(seq, protocol.EncodeHandshakeRes(seq, codes.OK))
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
	epoch uint64
	seq   uint64
}

// replyState 响应缓存条目状态
type replyState int

const (
	replyExecuting replyState = iota + 1 // 正在执行
	replyDone                            // 已完成
)

// replyEntry 响应缓存条目
type replyEntry struct {
	state  replyState
	done   chan struct{} // 执行完成信号
	data   []byte
	expire time.Time
}

// replyCache 响应缓存，用于重连后按 (insID, epoch, seq) 幂等去重
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

// begin 尝试标记请求为执行中；若已有相同请求在执行则返回其完成信号，若已完成则返回缓存响应
// 过期条目（含执行中条目）会被回收并重新授予执行权，避免异步回包缺失导致占位永久泄漏
func (c *replyCache) begin(insID string, epoch, seq uint64) (done chan struct{}, data []byte, executing bool) {
	c.mu.Lock()

	key := replyKey{insID: insID, epoch: epoch, seq: seq}

	if entry, ok := c.entries[key]; ok {
		if time.Now().Before(entry.expire) {
			switch entry.state {
			case replyExecuting:
				c.mu.Unlock()
				return entry.done, nil, false
			case replyDone:
				c.mu.Unlock()
				return nil, entry.data, false
			}
		}

		// 过期条目：若是执行中，唤醒旧等待者后回收
		if entry.state == replyExecuting {
			close(entry.done)
		}
		delete(c.entries, key)
	}

	entry := &replyEntry{state: replyExecuting, done: make(chan struct{}), expire: time.Now().Add(c.ttl)}
	c.entries[key] = entry
	c.mu.Unlock()

	return entry.done, nil, true
}

// get 获取已完成的缓存响应
func (c *replyCache) get(insID string, epoch, seq uint64) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[replyKey{insID: insID, epoch: epoch, seq: seq}]
	if !ok || entry.state != replyDone {
		return nil, false
	}

	if time.Now().After(entry.expire) {
		return nil, false
	}

	return entry.data, true
}

// finish 缓存响应并唤醒等待者；data 为空表示执行失败，缓存立即失效
// 重复调用安全：仅 executing 状态的条目会被转换并唤醒等待者
func (c *replyCache) finish(insID string, epoch, seq uint64, data []byte) {
	c.mu.Lock()

	key := replyKey{insID: insID, epoch: epoch, seq: seq}

	entry, ok := c.entries[key]
	if !ok || entry.state != replyExecuting {
		c.mu.Unlock()
		return
	}

	now := time.Now()

	// 惰性清理过期条目
	if now.After(c.nextSweep) {
		c.nextSweep = now.Add(c.ttl / 2)
		for k, e := range c.entries {
			if now.After(e.expire) {
				delete(c.entries, k)
			}
		}
	}

	if len(data) == 0 {
		delete(c.entries, key)
	} else {
		entry.state = replyDone
		entry.data = data
		entry.expire = now.Add(c.ttl)
	}

	close(entry.done)
	c.mu.Unlock()
}
