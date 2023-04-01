package grpc

import (
	"github.com/dobyte/due/internal/endpoint"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/grpc/gate"
	"github.com/dobyte/due/transport/grpc/internal/server"
	"github.com/dobyte/due/transport/grpc/node"
	"github.com/dobyte/due/transport/grpc/service"
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

// NewGateServer 新建网关服务器
func (t *Transporter) NewGateServer(provider transport.GateProvider) (transport.Server, error) {
	return gate.NewServer(provider, &t.opts.server)
}

// NewNodeServer 新建节点服务器
func (t *Transporter) NewNodeServer(provider transport.NodeProvider) (transport.Server, error) {
	return node.NewServer(provider, &t.opts.server)
}

// NewServiceServer 新建微服务服务器
func (t *Transporter) NewServiceServer() (transport.Server, error) {
	return server.NewServer(&t.opts.server)
}

// NewGateClient 新建网关客户端
func (t *Transporter) NewGateClient(ep *endpoint.Endpoint) (transport.GateClient, error) {
	return gate.NewClient(ep, &t.opts.client)
}

// NewNodeClient 新建节点客户端
func (t *Transporter) NewNodeClient(ep *endpoint.Endpoint) (transport.NodeClient, error) {
	return node.NewClient(ep, &t.opts.client)
}

// NewServiceClient 新建服务客户端
func (t *Transporter) NewServiceClient(target string) (transport.ServiceClient, error) {
	return service.NewClient(target, &t.opts.client)
}
