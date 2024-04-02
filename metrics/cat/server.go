package cat

import "github.com/cat-go/cat"

type Server struct {
	opts *options
}

func NewServer(opts ...Option) *Server {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	s := &Server{}
	s.opts = o
	return s
}

// Name 组件名称
func (s *Server) Name() string {
	return "cat"
}

// Init 初始化组件
func (s *Server) Init() {
}

// Start 启动组件
func (s *Server) Start() {
	if s.opts.enable {
		cat.Init(&cat.Options{
			AppId:      s.opts.name,
			ServerAddr: s.opts.addr,
			HttpPort:   s.opts.port,
		})
	}
}

// Destroy 销毁组件
func (s *Server) Destroy() {
	if s.opts.enable {
		cat.Shutdown()
	}
}

// Enable 获取是否开启
func (s *Server) Enable() bool {
	return s.opts.enable
}
