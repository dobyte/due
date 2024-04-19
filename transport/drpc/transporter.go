package drpc

import (
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/transport/drpc/gate"
	"github.com/dobyte/due/v2/transport/drpc/node"
	"sync"
)

type Transporter struct {
	opts    *options
	once1   sync.Once
	client1 transport.GateClient
	once2   sync.Once
	client2 transport.NodeClient
}

func NewTransporter(opts ...Option) *Transporter {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Transporter{opts: o}
}

// SetDefaultDiscovery 设置默认的服务发现组件
func (t *Transporter) SetDefaultDiscovery(discovery registry.Discovery) {

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
	return nil, nil
}

// NewGateClient 新建网关客户端
func (t *Transporter) NewGateClient(ep *endpoint.Endpoint) (transport.GateClient, error) {
	t.once1.Do(func() {
		t.client1 = gate.NewClient(ep)
	})

	return t.client1, nil
}

// NewNodeClient 新建节点客户端
func (t *Transporter) NewNodeClient(ep *endpoint.Endpoint) (transport.NodeClient, error) {
	t.once2.Do(func() {
		t.client2 = node.NewClient(ep)
	})

	return t.client2, nil
}

// NewServiceClient 新建微服务客户端
func (t *Transporter) NewServiceClient(target string) (transport.ServiceClient, error) {
	return nil, nil
}
