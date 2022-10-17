package grpc

import (
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/grpc/gate"
	"github.com/dobyte/due/transport/grpc/node"
)

type Transporter struct {
}

func NewTransporter() *Transporter {
	return &Transporter{}
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
func (t *Transporter) NewGateServer() (transport.GateServer, error) {

}

func (t *Transporter) NewNodeServer() {

}
