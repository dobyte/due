package transport

import "github.com/dobyte/due/internal/endpoint"

type Server interface {
	// Addr 监听地址
	Addr() string
	// Scheme 协议
	Scheme() string
	// Endpoint 服务端口
	Endpoint() *endpoint.Endpoint
	// Start 启动服务器
	Start() error
	// Stop 停止服务器
	Stop() error
}

type GateServer interface {
	Server
	Inject()
}
