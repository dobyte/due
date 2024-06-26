package grpc

import (
	"github.com/dobyte/due/transport/grpc/v2/internal/client"
	"github.com/dobyte/due/transport/grpc/v2/internal/server"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"sync"
)

const name = "grpc"

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

// NewServer 新建微服务服务器
func (t *Transporter) NewServer() (transport.Server, error) {
	return server.NewServer(&t.opts.server)
}

// NewClient 新建微服务客户端
func (t *Transporter) NewClient(target string) (transport.Client, error) {
	t.once.Do(func() {
		t.builder = client.NewBuilder(&t.opts.client)
	})

	cc, err := t.builder.Build(target)
	if err != nil {
		return nil, err
	}

	return client.NewClient(cc), nil
}
