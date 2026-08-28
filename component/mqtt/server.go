package mqtt

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/errors"
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
	hooks  []mqtt.HookLoadConfig
}

func NewServer(opts ...Option) *Server {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Server{}
	s.opts = o
	s.proxy = newProxy(s)

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
	if len(s.opts.listensOpts) == 0 {
		log.Fatalf("mqtt server listens opts is empty")
	}

	opts := &mqtt.Options{}
	opts.InlineClient = true
	opts.Capabilities = mqtt.NewDefaultServerCapabilities()
	opts.ClientNetReadBufferSize = s.opts.readBufferSize
	opts.ClientNetWriteBufferSize = s.opts.writeBufferSize
	opts.Listeners = make([]listeners.Config, 0, len(s.opts.listensOpts))
	opts.Hooks = make([]mqtt.HookLoadConfig, 0)
	opts.Logger = slog.New(log.GetLogger())

	for _, opt := range s.opts.listensOpts {
		if typ := strings.ToLower(opt.Type); typ != listeners.TypeWS && typ != listeners.TypeTCP {
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

		listenAddr, _, err := xnet.ParseAddr(opt.Addr)
		if err != nil {
			log.Fatalf("listener %s address parse failed: %v", opt.ID, err)
		}

		opts.Listeners = append(opts.Listeners, listeners.Config{
			ID:        opt.ID,
			Type:      opt.Type,
			Address:   listenAddr,
			TLSConfig: tlsConfig,
		})
	}

	if len(opts.Listeners) == 0 {
		log.Fatalf("mqtt server listens opts is empty")
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
		hasAuth, hasACL := s.hookCapabilities()
		if !hasAuth && !hasACL {
			opts.Hooks = append(opts.Hooks, mqtt.HookLoadConfig{
				Hook: &auth.AllowHook{},
			})
		} else if !hasAuth || !hasACL {
			log.Warnf("mqtt server auth or acl capability is missing: auth=%v acl=%v", hasAuth, hasACL)
		}
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

	for _, item := range s.hooks {
		opts.Hooks = append(opts.Hooks, item)
	}

	s.printInfo()

	s.server = mqtt.New(opts)

	go func() {
		if err := s.server.Serve(); err != nil {
			log.Fatalf("mqtt server startup failed: %v", err)
		}
	}()
}

// Destroy 销毁组件
func (s *Server) Destroy() {
	if s.server != nil {
		if err := s.server.Close(); err != nil {
			log.Warnf("mqtt server shutdown failed: %v", err)
		}
		s.server = nil
	}
}

// Proxy 获取MQTT代理API
// @return @1 *Proxy MQTT代理
func (s *Server) Proxy() *Proxy {
	return s.proxy
}

// 打印服务启动信息
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
		_, exposeAddr, err := xnet.ParseAddr(opt.Addr)
		if err != nil {
			continue
		}

		infos = append(infos, info.MakeHorizontalLine())
		infos = append(infos, fmt.Sprintf("ID: %s", opt.ID))
		infos = append(infos, fmt.Sprintf("Type: %s", opt.Type))

		switch opt.Type {
		case listeners.TypeTCP:
			infos = append(infos, fmt.Sprintf("Addr: %s", exposeAddr))
		case listeners.TypeWS:
			if opt.CertFile != "" && opt.KeyFile != "" {
				infos = append(infos, fmt.Sprintf("Addr: %s", fmt.Sprintf("wss://%s", exposeAddr)))
			} else {
				infos = append(infos, fmt.Sprintf("Addr: %s", fmt.Sprintf("ws://%s", exposeAddr)))
			}
		}
	}

	info.PrintBoxInfo("MQTT", infos...)
}

// 添加Hook
// 服务启动前添加的Hook会在启动时加载，服务启动后添加返回错误
// @param hook Hook 待添加的Hook
// @param config ...any 可选，Hook配置
// @return @1 error 服务已启动时返回的错误
func (s *Server) addHook(hook Hook, config ...any) error {
	if s.server == nil {
		if len(config) > 0 {
			s.hooks = append(s.hooks, mqtt.HookLoadConfig{
				Hook:   hook,
				Config: config[0],
			})
		} else {
			s.hooks = append(s.hooks, mqtt.HookLoadConfig{
				Hook: hook,
			})
		}

		return nil
	} else {
		return errors.ErrServerStarted
	}
}

// 检测已注册Hook提供的认证与ACL能力
// @return @1 bool 是否存在提供认证能力（OnConnectAuthenticate）的Hook
// @return @2 bool 是否存在提供ACL能力（OnACLCheck）的Hook
func (s *Server) hookCapabilities() (hasAuth, hasACL bool) {
	for _, item := range s.hooks {
		hasAuth = hasAuth || item.Hook.Provides(mqtt.OnConnectAuthenticate)
		hasACL = hasACL || item.Hook.Provides(mqtt.OnACLCheck)
	}
	return
}
