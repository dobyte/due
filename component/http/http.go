package http

import (
	"fmt"
	"github.com/dobyte/due/component/http/v2/swagger"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/log"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"strings"
)

type Http struct {
	component.Base
	opts  *options
	app   *fiber.App
	proxy *Proxy
}

func NewHttp(opts ...Option) *Http {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	h := &Http{}
	h.opts = o
	h.proxy = newProxy(h)
	h.app = fiber.New(fiber.Config{
		ServerHeader: o.name,
	})
	h.app.Use(logger.New())
	h.app.Use(recover.New())
	if h.opts.swagger.Enable {
		h.app.Use(swagger.New(swagger.Config{
			Title:    h.opts.swagger.Title,
			BasePath: h.opts.swagger.BasePath,
			FilePath: h.opts.swagger.FilePath,
		}))
	}

	return h
}

// Name 组件名称
func (h *Http) Name() string {
	return h.opts.name
}

// Init 初始化组件
func (h *Http) Init() {}

// Proxy 获取HTTP代理API
func (h *Http) Proxy() *Proxy {
	return h.proxy
}

// Start 启动组件
func (h *Http) Start() {
	listenAddr, exposeAddr, err := xnet.ParseAddr(h.opts.addr)
	if err != nil {
		log.Fatalf("http addr parse failed: %v", err)
	}

	if h.opts.transporter != nil && h.opts.registry != nil {
		h.opts.transporter.SetDefaultDiscovery(h.opts.registry)
	}

	h.printInfo(exposeAddr)

	go func() {
		if err = h.app.Listen(listenAddr, fiber.ListenConfig{
			CertFile:              h.opts.certFile,
			CertKeyFile:           h.opts.keyFile,
			DisableStartupMessage: true,
		}); err != nil {
			log.Fatal("http server startup failed: %v", err)
		}
	}()
}

func (h *Http) printInfo(addr string) {
	infos := make([]string, 0, 3)
	infos = append(infos, fmt.Sprintf("Name: %s", h.Name()))

	var baseUrl string
	if h.opts.certFile != "" && h.opts.keyFile != "" {
		baseUrl = fmt.Sprintf("https://%s", addr)
	} else {
		baseUrl = fmt.Sprintf("http://%s", addr)
	}

	infos = append(infos, fmt.Sprintf("Url: %s", baseUrl))

	if h.opts.swagger.Enable {
		infos = append(infos, fmt.Sprintf("Swagger: %s/%s", baseUrl, strings.TrimPrefix(h.opts.swagger.BasePath, "/")))
	}

	if h.opts.registry != nil {
		infos = append(infos, fmt.Sprintf("Registry: %s", h.opts.registry.Name()))
	} else {
		infos = append(infos, "Registry: -")
	}

	if h.opts.transporter != nil {
		infos = append(infos, fmt.Sprintf("Transporter: %s", h.opts.transporter.Name()))
	} else {
		infos = append(infos, "Transporter: -")
	}

	info.PrintBoxInfo("Http", infos...)
}
