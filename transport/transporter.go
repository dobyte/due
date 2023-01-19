package transport

import (
	"github.com/dobyte/due/internal/endpoint"
)

type Transporter interface {
	// NewGateServer 新建网关服务器
	NewGateServer(provider GateProvider) (Server, error)
	// NewNodeServer 新建节点服务器
	NewNodeServer(provider NodeProvider) (Server, error)
	// NewGateClient 新建网关客户端
	NewGateClient(ep *endpoint.Endpoint) (GateClient, error)
	// NewNodeClient 新建节点客户端
	NewNodeClient(ep *endpoint.Endpoint) (NodeClient, error)
}
