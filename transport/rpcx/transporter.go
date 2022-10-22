package rpcx

import (
	"github.com/dobyte/due/router"
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
func (t *Transporter) NewGateClient(ep *router.Endpoint) (transport.GateClient, error) {
	return gate.NewClient(ep)
}

// NewNodeClient 新建节点客户端
func (t *Transporter) NewNodeClient(ep *router.Endpoint) (transport.NodeClient, error) {
	return node.NewClient(ep)
}

// NewGateServer 新建网关服务器
func (t *Transporter) NewGateServer(provider transport.GateProvider) (transport.Server, error) {
	return gate.NewServer(provider, &server.Options{
		Addr: t.opts.server.addr,
	})
}

// NewNodeServer 新建节点服务器
func (t *Transporter) NewNodeServer(provider transport.NodeProvider) (transport.Server, error) {
	return node.NewServer(provider, &server.Options{
		Addr: t.opts.server.addr,
	})
}
