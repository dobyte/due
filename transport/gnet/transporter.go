package gnet

import (
	"github.com/symsimmy/due/common/endpoint"
	"github.com/symsimmy/due/registry"
	"github.com/symsimmy/due/transport"
	"github.com/symsimmy/due/transport/gnet/gate"
	"github.com/symsimmy/due/transport/gnet/node"
	"github.com/symsimmy/due/transport/gnet/tcp"
	"sync"
)

type Transporter struct {
	opts    *options
	once    sync.Once
	builder *tcp.Builder
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

// NewGateClient 新建网关客户端
func (t *Transporter) NewGateClient(ep *endpoint.Endpoint) (transport.GateClient, error) {
	t.once.Do(func() {
		t.builder = tcp.NewBuilder()
	})

	cc, err := t.builder.Build(ep.Address())
	if err != nil {
		return nil, err
	}

	return gate.NewClient(cc), nil
}

// NewNodeClient 新建节点客户端
func (t *Transporter) NewNodeClient(ep *endpoint.Endpoint) (transport.NodeClient, error) {
	t.once.Do(func() {
		t.builder = tcp.NewBuilder()
	})

	cc, err := t.builder.Build(ep.Address())
	if err != nil {
		return nil, err
	}

	return node.NewClient(cc), nil
}

func (t *Transporter) UpdateClientPool(endpoints map[string]*endpoint.Endpoint) {
	t.once.Do(func() {
		t.builder = tcp.NewBuilder()
	})

	endpointMap := make(map[string]bool, len(endpoints))
	for _, value := range endpoints {
		endpointMap[value.Address()] = true
	}

	t.builder.Update(endpointMap)
}
