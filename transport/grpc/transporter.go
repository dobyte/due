package grpc

import (
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/grpc/gate"
	"github.com/dobyte/due/transport/grpc/internal/client"
	"github.com/dobyte/due/transport/grpc/internal/server"
	"github.com/dobyte/due/transport/grpc/node"
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
	return gate.NewClient(ep, &client.Options{
		CertFile:   t.opts.client.certFile,
		ServerName: t.opts.client.serverName,
	})
}

// NewNodeClient 新建节点客户端
func (t *Transporter) NewNodeClient(ep *endpoint.Endpoint) (transport.NodeClient, error) {
	return node.NewClient(ep, &client.Options{
		CertFile:   t.opts.client.certFile,
		ServerName: t.opts.client.serverName,
	})
}

// NewGateServer 新建网关服务器
func (t *Transporter) NewGateServer(provider transport.GateProvider) (transport.Server, error) {
	return gate.NewServer(provider, &server.Options{
		Addr:       t.opts.server.addr,
		KeyFile:    t.opts.server.keyFile,
		CertFile:   t.opts.server.certFile,
		ServerOpts: t.opts.server.serverOpts,
	})
}

// NewNodeServer 新建节点服务器
func (t *Transporter) NewNodeServer(provider transport.NodeProvider) (transport.Server, error) {
	return node.NewServer(provider, &server.Options{
		Addr:       t.opts.server.addr,
		KeyFile:    t.opts.server.keyFile,
		CertFile:   t.opts.server.certFile,
		ServerOpts: t.opts.server.serverOpts,
	})
}
