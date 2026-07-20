package mqtt

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"

	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/log"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/hooks/debug"
	"github.com/mochi-mqtt/server/v2/listeners"
)

type Server struct {
	component.Base
	opts   *options
	proxy  *Proxy
	server *mqtt.Server
	events map[Event]EventHandler
}

func NewServer(opts ...Option) *Server {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Server{}
	s.opts = o
	s.proxy = newProxy(s)
	s.events = make(map[Event]EventHandler)

	return s
}

// Name 组件名称
func (s *Server) Name() string {
	return s.opts.name
}

// Init 初始化组件
func (s *Server) Init() {}

// Start 启动组件
func (s *Server) Start() {
	opts := &mqtt.Options{}
	opts.InlineClient = true
	opts.Capabilities = mqtt.NewDefaultServerCapabilities()
	opts.ClientNetReadBufferSize = s.opts.readBufferSize
	opts.ClientNetWriteBufferSize = s.opts.writeBufferSize
	opts.Listeners = make([]listeners.Config, 0, len(s.opts.listensOpts))
	opts.Hooks = make([]mqtt.HookLoadConfig, 0)
	opts.Logger = slog.New(log.GetLogger())

	for _, opt := range s.opts.listensOpts {
		if opt.Type != listeners.TypeWS && opt.Type != listeners.TypeTCP {
			continue
		}

		var tlsConfig *tls.Config

		if opt.CertFile != "" && opt.KeyFile != "" {
			if cert, err := tls.LoadX509KeyPair(opt.CertFile, opt.KeyFile); err != nil {
				log.Fatalf("listener %s tls certificate load failed: %v", opt.ID, err)
			} else {
				tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
			}
		}

		if listenAddr, exposeAddr, err := xnet.ParseAddr(opt.Addr); err != nil {
			log.Fatalf("listener %s address parse failed: %v", opt.ID, err)
		} else {
			opt.listenAddr, opt.exposeAddr = listenAddr, exposeAddr
		}

		opts.Listeners = append(opts.Listeners, listeners.Config{
			ID:        opt.ID,
			Type:      opt.Type,
			Address:   opt.listenAddr,
			TLSConfig: tlsConfig,
		})
	}

	if s.opts.auth != "" {
		data, err := os.ReadFile(s.opts.auth)
		if err != nil {
			log.Fatalf("auth file read failed: %v", err)
		}

		opts.Hooks = append(opts.Hooks, mqtt.HookLoadConfig{
			Hook:   &auth.Hook{},
			Config: &auth.Options{Data: data},
		})
	} else {
		opts.Hooks = append(opts.Hooks, mqtt.HookLoadConfig{
			Hook: &auth.AllowHook{},
		})
	}

	if s.opts.debug {
		opts.Hooks = append(opts.Hooks, mqtt.HookLoadConfig{
			Hook: &debug.Hook{},
			Config: &debug.Options{
				Enable:         true,
				ShowPacketData: true,
				ShowPings:      true,
				ShowPasswords:  true,
			},
		})
	}

	opts.Hooks = append(opts.Hooks, mqtt.HookLoadConfig{
		Hook: &eventHook{server: s},
	})

	s.server = mqtt.New(opts)

	go func() {
		if err := s.server.Serve(); err != nil {
			log.Fatalf("mqtt server startup failed: %v", err)
		}
	}()

	s.printInfo()
}

// Destroy 销毁组件
func (s *Server) Destroy() {
	if s.server != nil {
		if err := s.server.Close(); err != nil {
			log.Warnf("mqtt server shutdown failed: %v", err)
		}
	}
}

// Proxy 获取MQTT代理API
func (s *Server) Proxy() *Proxy {
	return s.proxy
}

func (s *Server) printInfo() {
	infos := make([]string, 0, 3)
	infos = append(infos, fmt.Sprintf("Name: %s", s.Name()))

	if s.opts.auth != "" {
		infos = append(infos, fmt.Sprintf("Auth: %s", s.opts.auth))
	} else {
		infos = append(infos, "Auth: allow")
	}

	if s.opts.debug {
		infos = append(infos, "Debug: true")
	} else {
		infos = append(infos, "Debug: false")
	}

	for _, opt := range s.opts.listensOpts {
		infos = append(infos, info.MakeHorizontalLine())
		infos = append(infos, fmt.Sprintf("ID: %s", opt.ID))
		infos = append(infos, fmt.Sprintf("Type: %s", opt.Type))

		switch opt.Type {
		case listeners.TypeTCP:
			infos = append(infos, fmt.Sprintf("Addr: %s", opt.exposeAddr))
		case listeners.TypeWS:
			if opt.CertFile != "" && opt.KeyFile != "" {
				infos = append(infos, fmt.Sprintf("Addr: %s", fmt.Sprintf("wss://%s", opt.exposeAddr)))
			} else {
				infos = append(infos, fmt.Sprintf("Addr: %s", fmt.Sprintf("ws://%s", opt.exposeAddr)))
			}
		}
	}

	info.PrintBoxInfo("MQTT", infos...)
}

// 添加事件处理器
func (s *Server) addEventHandler(event Event, handler EventHandler) {
	if s.server == nil {
		s.events[event] = handler
	}
}
