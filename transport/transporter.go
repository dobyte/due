package transport

import "github.com/dobyte/due/router"

type Transporter interface {
	NewGateServer()
	// NewGateClient 新建网关客户端
	NewGateClient(ep *router.Endpoint) (GateClient, error)
	// NewNodeClient 新建节点客户端
	NewNodeClient(ep *router.Endpoint) (NodeClient, error)
}
