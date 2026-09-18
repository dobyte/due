package drpc

import (
	"context"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/endpoint"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	taskpool "github.com/dobyte/due/v2/task"
)

const scheme = "drpc"

type Server struct {
	opts       *ServerOptions     // 配置
	listenAddr string             // 监听地址
	exposeAddr string             // 暴露地址
	endpoint   *endpoint.Endpoint // 暴露端点
	mu         sync.Mutex         // 锁
	ctx        context.Context    // 上下文
	cancel     context.CancelFunc // 取消函数
	listener   *net.TCPListener   // 监听器
	handlers   [256]RouteHandler  // 路由处理器
	conns      sync.Map           // 连接映射
	ticker     *time.Ticker       // 心跳定时器
	queues     sync.Map           // 已关闭队列
	workers    []*ServerWorker    // 工作协程
	connSeq    atomic.Uint64      // 连接序号
	workerWg   sync.WaitGroup     // 工作协程等待组
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

	if err := s.init(); err != nil {
		return err
	}

	go s.serve(s.listener)
	go s.check()

	return nil
}

// Stop 关闭服务器
// @return @1 error 错误信息
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return errors.ErrServerClosed
	}

	if err := s.listener.Close(); err != nil {
		log.Warnf("tcp listener close error: %v", err)
	}

	s.cancel()
	s.listener = nil
	s.ticker.Stop()
	s.closeAllConns()
	s.clearAllQueues()
	s.closeWorkers()

	return nil
}

// RegisterHandler 注册处理器
func (s *Server) RegisterHandler(route uint8, handler RouteHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		log.Warnf("server already started, cannot register handler: %d", route)
		return
	}

	if s.handlers[route] != nil {
		log.Warnf("handler already registered for route: %d", route)
		return
	}

	s.handlers[route] = handler
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

	_ = s.Stop()
}

// init 初始化服务器
func (s *Server) init() error {
	if s.listener != nil {
		return errors.ErrServerStarted
	}

	addr, err := net.ResolveTCPAddr("tcp", s.listenAddr)
	if err != nil {
		return err
	}

	if s.listener, err = net.ListenTCP(addr.Network(), addr); err != nil {
		return err
	}

	s.ticker = time.NewTicker(heartbeatInterval)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.initWorkers()

	return nil
}

// check 检查连接是否超时
func (s *Server) check() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case t, ok := <-s.ticker.C:
			if !ok {
				return
			}

			taskpool.Add(func() {
				s.conns.Range(func(_, cc any) bool {
					cc.(*ServerConn).checkHeartbeat(&t)
					return true
				})
			})

			taskpool.Add(func() {
				s.queues.Range(func(k, v any) bool {
					q := v.(*closedQueue)

					if q.time.Add(maxRetentionTime).Before(time.Now()) {
						if s.queues.CompareAndDelete(k, v) {
							for buf := range q.queue.Read() {
								buf.Release()
							}
						}
					}

					return true
				})
			})
		}
	}
}

// handleMessage 处理消息
// @param conn 连接
// @param route 路由
// @param seq 序列号
// @param buf 消息缓冲区
// @return @1 error 错误信息
func (s *Server) handleMessage(conn *ServerConn, route uint8, seq uint64, buf *buffer.Bytes) error {
	if handler := s.handlers[route]; handler == nil {
		buf.Release()

		return errors.ErrNotFoundRoute
	} else {
		return handler(conn, seq, buf)
	}
}

// initWorkers 初始化工作协程
func (s *Server) initWorkers() {
	num := s.opts.WorkerNum
	if num <= 0 {
		num = int32(runtime.GOMAXPROCS(0))
	}

	size := max(128, int(s.opts.WriteQueueSize))

	s.workers = make([]*ServerWorker, 0, int(num))
	for i := 0; i < int(num); i++ {
		w := &ServerWorker{svr: s, tasks: make(chan *serverTask, size)}
		s.workers = append(s.workers, w)
		s.workerWg.Add(1)
		go w.run()
	}
}

// allocateWorker 分配工作协程
func (s *Server) allocateWorker() *ServerWorker {
	return s.workers[(s.connSeq.Add(1)-1)%uint64(len(s.workers))]
}

// closeWorkers 关闭工作协程
func (s *Server) closeWorkers() {
	for _, w := range s.workers {
		close(w.tasks)
	}

	s.workerWg.Wait()
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
func (s *Server) closeAllConns() {
	wg, _ := taskpool.WithContext(context.Background())

	s.conns.Range(func(_, cc any) bool {
		wg.Go(cc.(*ServerConn).forceClose)
		return true
	})

	wg.Wait()
}

// 清除所有已关闭队列
func (s *Server) clearAllQueues() {
	wg, _ := taskpool.WithContext(context.Background())

	s.queues.Range(func(k, v any) bool {
		if s.queues.CompareAndDelete(k, v) {
			q := v.(*closedQueue)

			wg.Go(func() error {
				for buf := range q.queue.Read() {
					buf.Release()
				}
				return nil
			})
		}

		return true
	})

	wg.Wait()
}

// 加载队列
func (s *Server) doLoadQueue(key string) (*queue.Queue[buffer.Buffer], bool) {
	if v, ok := s.queues.LoadAndDelete(key); ok {
		q := v.(*closedQueue)

		if q.time.Add(maxRetentionTime).After(time.Now()) {
			return q.queue, true
		} else {
			for buf := range q.queue.Read() {
				buf.Release()
			}
		}
	}

	return nil, false
}

// 缓存队列
func (s *Server) doCacheQueue(key string, queue *queue.Queue[buffer.Buffer]) {
	if v, ok := s.queues.Swap(key, &closedQueue{
		queue: queue,
		time:  time.Now(),
	}); ok {
		q := v.(*closedQueue)

		for buf := range q.queue.Read() {
			buf.Release()
		}
	}
}
