package http

import (
	stctx "context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dobyte/due/component/http/v2/swagger"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

// Server HTTP服务器
// 基于fiber框架实现的HTTP服务组件，支持路由注册、中间件、跨域、Swagger等能力
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
	s.app = fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
		return newContext(fiber.NewDefaultCtx(app), s.proxy)
	}, fiber.Config{
		ServerHeader:                 o.name,
		StrictRouting:                o.strictRouting,
		CaseSensitive:                o.caseSensitive,
		DisableHeadAutoRegister:      o.disableHeadAutoRegister,
		Immutable:                    o.immutable,
		UnescapePath:                 o.unescapePath,
		BodyLimit:                    o.bodyLimit,
		Concurrency:                  o.concurrency,
		Views:                        o.views,
		ViewsLayout:                  o.viewsLayout,
		PassLocalsToViews:            o.passLocalsToViews,
		ReadBufferSize:               o.readBufferSize,
		WriteBufferSize:              o.writeBufferSize,
		ProxyHeader:                  o.proxyHeader,
		ErrorHandler:                 o.errorHandler,
		DisableKeepalive:             o.disableKeepalive,
		DisableDefaultDate:           o.disableDefaultDate,
		DisableDefaultContentType:    o.disableDefaultContentType,
		DisableHeaderNormalizing:     o.disableHeaderNormalizing,
		StreamRequestBody:            o.streamRequestBody,
		DisablePreParseMultipartForm: o.disablePreParseMultipartForm,
		ReduceMemoryUsage:            o.reduceMemoryUsage,
		EnableIPValidation:           o.enableIPValidation,
		EnableSplittingOnParsers:     o.enableSplittingOnParsers,
		TrustProxy:                   o.trustProxy,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies:   o.trustProxyConfig.Proxies,
			LinkLocal: o.trustProxyConfig.LinkLocal,
			Loopback:  o.trustProxyConfig.Loopback,
			Private:   o.trustProxyConfig.Private,
		},
	})

	if o.console {
		s.app.Use(logger.New())
	}

	s.app.Use(recover.New(recover.Config{EnableStackTrace: !mode.IsReleaseMode()}))

	if s.opts.corsOpts.Enable {
		s.app.Use(cors.New(cors.Config{
			AllowOrigins:        s.opts.corsOpts.AllowOrigins,
			AllowMethods:        s.opts.corsOpts.AllowMethods,
			AllowHeaders:        s.opts.corsOpts.AllowHeaders,
			AllowCredentials:    s.opts.corsOpts.AllowCredentials,
			ExposeHeaders:       s.opts.corsOpts.ExposeHeaders,
			MaxAge:              s.opts.corsOpts.MaxAge,
			AllowPrivateNetwork: s.opts.corsOpts.AllowPrivateNetwork,
		}))
	}

	if s.opts.swagOpts.Enable {
		if middleware := swagger.New(swagger.Config{
			Title:            s.opts.swagOpts.Title,
			BasePath:         s.opts.swagOpts.BasePath,
			FilePath:         s.opts.swagOpts.FilePath,
			SwaggerBundleUrl: s.opts.swagOpts.SwaggerBundleUrl,
			SwaggerPresetUrl: s.opts.swagOpts.SwaggerPresetUrl,
			SwaggerStylesUrl: s.opts.swagOpts.SwaggerStylesUrl,
		}); middleware != nil {
			s.app.Use(middleware)
		} else {
			s.opts.swagOpts.Enable = false
		}
	}

	for i := range o.middlewares {
		if handler, ok := o.middlewares[i].(Handler); ok {
			s.app.Use(func(ctx fiber.Ctx) error {
				return handler(ctx.(Context))
			})
		} else {
			s.app.Use(handler)
		}
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

	go func() {
		if err = s.app.Listen(listenAddr, fiber.ListenConfig{
			CertFile:              s.opts.certFile,
			CertKeyFile:           s.opts.keyFile,
			DisableStartupMessage: true,
		}); err != nil {
			log.Fatalf("http server startup failed: %v", errors.Unwrap(errors.Unwrap(err)))
		}
	}()

	s.printInfo(exposeAddr)
}

// Destroy 销毁组件
func (s *Server) Destroy() {
	if s.app != nil {
		ctx, cancel := stctx.WithTimeout(stctx.Background(), 10*time.Second)
		err := s.app.ShutdownWithContext(ctx)
		cancel()

		if err != nil {
			log.Warnf("http server shutdown failed: %v", err)
		}
	}
}

// 打印服务启动信息
// @param addr string 对外暴露的服务地址
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

	if s.opts.swagOpts.Enable {
		infos = append(infos, fmt.Sprintf("Swagger: %s/%s", baseUrl, strings.TrimPrefix(s.opts.swagOpts.BasePath, "/")))
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
