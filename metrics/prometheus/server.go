package prometheus

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/symsimmy/due/log"
	"net/http"
	"strings"
)

type PromServer struct {
	engine *gin.Engine
	opts   *options
}

func NewPromServer(opts ...Option) *PromServer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	s := &PromServer{engine: gin.Default()}
	s.opts = o
	return s
}

// Name 组件名称
func (s *PromServer) Name() string {
	return "prometheus"
}

// Init 初始化组件
func (s *PromServer) Init() {
}

// Start 启动组件
func (s *PromServer) Start() {
	if s.opts != nil && s.opts.enable {
		go func() {
			if err := s.engine.Run(s.opts.addr); err != nil {
				log.Errorf("http server startup failed: %v", err)
			}
		}()
		s.engine.GET("/metrics", s.promHandler(promhttp.Handler()))
	}
}

func (s *PromServer) promHandler(handler http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

// Destroy 销毁组件
func (s *PromServer) Destroy() {

}

// Enable 获取是否开启
func (s *PromServer) Enable() bool {
	if s.opts == nil {
		return false
	}
	return s.opts.enable
}

// GetMetricsPort 获取metrics服务端口
func (s *PromServer) GetMetricsPort() string {
	addr := strings.Split(s.opts.addr, ":")
	if len(addr) == 2 {
		return addr[1]
	} else {
		return ""
	}
}
