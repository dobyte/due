package drpc

import (
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/transport/drpc/gate"
)

type Transporter struct {
	opts *options
	cli1 transport.GateClient
	cli2 transport.NodeClient
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
	t.opts.server.Addr = ":3553"
	return gate.NewServer(provider, &t.opts.server)
}

// NewNodeServer 新建节点服务器
func (t *Transporter) NewNodeServer(provider transport.NodeProvider) (transport.Server, error) {
	//return node.NewServer(provider, &t.opts.server)
	return nil, nil

}

// NewServiceServer 新建微服务服务器
func (t *Transporter) NewServiceServer() (transport.Server, error) {
	return nil, nil
}

// NewGateClient 新建网关客户端
func (t *Transporter) NewGateClient(ep *endpoint.Endpoint) (transport.GateClient, error) {
	return gate.NewClient(), nil
}

// NewNodeClient 新建节点客户端
func (t *Transporter) NewNodeClient(ep *endpoint.Endpoint) (transport.NodeClient, error) {
	return nil, nil
}

// NewServiceClient 新建微服务客户端
func (t *Transporter) NewServiceClient(target string) (transport.ServiceClient, error) {
	return nil, nil
}
