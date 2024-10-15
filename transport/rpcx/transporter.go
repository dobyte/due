package rpcx

import (
	"github.com/dobyte/due/transport/rpcx/v2/internal/client"
	"github.com/dobyte/due/transport/rpcx/v2/internal/logger"
	"github.com/dobyte/due/transport/rpcx/v2/internal/server"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"sync"
)

const name = "rpcx"

type Transporter struct {
	opts    *options
	once    sync.Once
	builder *client.Builder
}

func NewTransporter(opts ...Option) *Transporter {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	logger.InitLogger()

	return &Transporter{opts: o}
}

// Name 获取传输器组件名
func (t *Transporter) Name() string {
	return name
}

// SetDefaultDiscovery 设置默认的服务发现组件
func (t *Transporter) SetDefaultDiscovery(discovery registry.Discovery) {
	if t.opts.client.Discovery == nil {
		t.opts.client.Discovery = discovery
	}
}

// NewServer 新建传输服务器
func (t *Transporter) NewServer() (transport.Server, error) {
	return server.NewServer(&t.opts.server)
}

// NewClient 新建传输客户端
// target参数可分为三种模式:
// 服务直连模式: 	direct://127.0.0.1:8011
// 服务直连模式: 	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
// 服务发现模式: 	discovery://service_name
func (t *Transporter) NewClient(target string) (transport.Client, error) {
	t.once.Do(func() {
		t.builder = client.NewBuilder(&t.opts.client)
	})

	cli, err := t.builder.Build(target)
	if err != nil {
		return nil, err
	}

	return client.NewClient(cli), nil
}
