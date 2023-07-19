package transport

import (
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/registry"
)

type Transporter interface {
	// SetDefaultDiscovery 设置默认的服务发现组件
	SetDefaultDiscovery(discovery registry.Discovery)
	// NewGateServer 新建网关服务器
	NewGateServer(provider GateProvider) (Server, error)
	// NewNodeServer 新建节点服务器
	NewNodeServer(provider NodeProvider) (Server, error)
	// NewServiceServer 新建微服务服务器
	NewServiceServer() (Server, error)
	// NewGateClient 新建网关客户端
	NewGateClient(ep *endpoint.Endpoint) (GateClient, error)
	// NewNodeClient 新建节点客户端
	NewNodeClient(ep *endpoint.Endpoint) (NodeClient, error)
	// NewServiceClient 新建微服务客户端
	NewServiceClient(target string) (ServiceClient, error)
}
