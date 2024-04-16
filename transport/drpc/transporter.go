package drpc

import (
	"github.com/dobyte/due/v2/transport"
	"sync"
)

type Transporter struct {
	opts *options
	once sync.Once
	//builder *client.Builder
	once1 sync.Once
	cli1  transport.GateClient
	once2 sync.Once
	cli2  transport.NodeClient
}

func NewTransporter(opts ...Option) *Transporter {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Transporter{opts: o}
}

// NewGateServer 新建网关服务器
func (t *Transporter) NewGateServer(provider transport.GateProvider) (transport.Server, error) {
	return gate.NewServer(provider, &t.opts.server)
}

// NewNodeServer 新建节点服务器
func (t *Transporter) NewNodeServer(provider transport.NodeProvider) (transport.Server, error) {
	return node.NewServer(provider, &t.opts.server)
}
