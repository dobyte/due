package http

import (
	"fmt"
	"github.com/dobyte/due/component/http/v2/swagger"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/log"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"strings"
)

type Server struct {
	component.Base
	opts  *options
	app   *fiber.App
	proxy *Proxy
}

func NewServer(opts ...Option) *Server {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Server{}
	s.opts = o
	s.proxy = newProxy(s)
	s.app = fiber.New(fiber.Config{ServerHeader: o.name})
	s.app.Use(logger.New())
	s.app.Use(recover.New())
	s.app.Use(cors.New())

	for i := range o.middlewares {
		switch handler := o.middlewares[i].(type) {
		case Handler:
			s.app.Use(func(ctx fiber.Ctx) error {
				return handler(&context{Ctx: ctx})
			})
		case fiber.Handler:
			s.app.Use(handler)
		}
	}

	if s.opts.swagger.Enable {
		s.app.Use(swagger.New(swagger.Config{
			Title:    s.opts.swagger.Title,
			BasePath: s.opts.swagger.BasePath,
			FilePath: s.opts.swagger.FilePath,
		}))
	}

	return s
}

// Name 组件名称
func (s *Server) Name() string {
	return s.opts.name
}

// Init 初始化组件
func (s *Server) Init() {}

// Proxy 获取HTTP代理API
func (s *Server) Proxy() *Proxy {
	return s.proxy
}

// Start 启动组件
func (s *Server) Start() {
	listenAddr, exposeAddr, err := xnet.ParseAddr(s.opts.addr)
	if err != nil {
		log.Fatalf("http addr parse failed: %v", err)
	}

	if s.opts.transporter != nil && s.opts.registry != nil {
		s.opts.transporter.SetDefaultDiscovery(s.opts.registry)
	}

	s.printInfo(exposeAddr)

	go func() {
		if err = s.app.Listen(listenAddr, fiber.ListenConfig{
			CertFile:              s.opts.certFile,
			CertKeyFile:           s.opts.keyFile,
			DisableStartupMessage: true,
		}); err != nil {
			log.Fatal("http server startup failed: %v", err)
		}
	}()
}

func (s *Server) printInfo(addr string) {
	infos := make([]string, 0, 3)
	infos = append(infos, fmt.Sprintf("Name: %s", s.Name()))

	var baseUrl string
	if s.opts.certFile != "" && s.opts.keyFile != "" {
		baseUrl = fmt.Sprintf("https://%s", addr)
	} else {
		baseUrl = fmt.Sprintf("http://%s", addr)
	}

	infos = append(infos, fmt.Sprintf("Url: %s", baseUrl))

	if s.opts.swagger.Enable {
		infos = append(infos, fmt.Sprintf("Swagger: %s/%s", baseUrl, strings.TrimPrefix(s.opts.swagger.BasePath, "/")))
	}

	if s.opts.registry != nil {
		infos = append(infos, fmt.Sprintf("Registry: %s", s.opts.registry.Name()))
	} else {
		infos = append(infos, "Registry: -")
	}

	if s.opts.transporter != nil {
		infos = append(infos, fmt.Sprintf("Transporter: %s", s.opts.transporter.Name()))
	} else {
		infos = append(infos, "Transporter: -")
	}

	info.PrintBoxInfo("Http", infos...)
}
