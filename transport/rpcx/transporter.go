package rpcx

import (
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/rpcx/gate"
	"github.com/dobyte/due/transport/rpcx/internal/server"
	"github.com/dobyte/due/transport/rpcx/node"
)

type Transporter struct {
	opts *options
}

func NewTransporter(opts ...Option) *Transporter {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Transporter{opts: o}
}

// NewGateClient 新建网关客户端
func (t *Transporter) NewGateClient(ep *endpoint.Endpoint) (transport.GateClient, error) {
	return gate.NewClient(ep)
}

// NewNodeClient 新建节点客户端
func (t *Transporter) NewNodeClient(ep *endpoint.Endpoint) (transport.NodeClient, error) {
	return node.NewClient(ep)
}

// NewServer 新建普通服务器
func (t *Transporter) NewServer() (transport.Server, error) {
	return server.NewServer(&t.opts.server)
}

// NewGateServer 新建网关服务器
func (t *Transporter) NewGateServer(provider transport.GateProvider) (transport.Server, error) {
	return gate.NewServer(provider, &t.opts.server)
}

// NewNodeServer 新建节点服务器
func (t *Transporter) NewNodeServer(provider transport.NodeProvider) (transport.Server, error) {
	return node.NewServer(provider, &t.opts.server)
}
