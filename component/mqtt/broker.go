package mqtt

import (
	"crypto/tls"
	"fmt"
	"os"

	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/core/info"
	xnet "github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/hooks/debug"
	"github.com/mochi-mqtt/server/v2/listeners"
)

type Broker struct {
	component.Base
	opts   *options
	server *mqtt.Server
}

func NewBroker(opts ...Option) *Broker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Broker{
		opts: o,
	}
}

// Name 组件名称
func (b *Broker) Name() string {
	return b.opts.name
}

// Init 初始化组件
func (b *Broker) Init() {}

// Start 启动组件
func (b *Broker) Start() {
	opts := &mqtt.Options{}
	opts.Capabilities = mqtt.NewDefaultServerCapabilities()
	opts.ClientNetReadBufferSize = b.opts.readBufferSize
	opts.ClientNetWriteBufferSize = b.opts.writeBufferSize
	opts.Listeners = make([]listeners.Config, 0, len(b.opts.listensOpts))
	opts.Hooks = make([]mqtt.HookLoadConfig, 0)

	for _, opt := range b.opts.listensOpts {
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

	if b.opts.authFile != "" {
		data, err := os.ReadFile(b.opts.authFile)
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

	if b.opts.debug && !mode.IsReleaseMode() {
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

	b.server = mqtt.New(opts)

	go func() {
		if err := b.server.Serve(); err != nil {
			log.Fatalf("mqtt server startup failed: %v", err)
		}
	}()

	b.printInfo()
}

// Destroy 销毁组件
func (b *Broker) Destroy() {
	if b.server != nil {
		if err := b.server.Close(); err != nil {
			log.Warnf("mqtt server shutdown failed: %v", err)
		}
	}
}

func (s *Broker) printInfo() {
	infos := make([]string, 0, 3)
	infos = append(infos, fmt.Sprintf("Name: %s", s.Name()))

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
