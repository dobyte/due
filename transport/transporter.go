package transport

import (
	"context"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/registry"
)

type Server interface {
	// Start 启动服务器
	Start() error
	// Stop 停止服务器
	Stop() error
	// Addr 监听地址
	Addr() string
	// Scheme 协议
	Scheme() string
	// Endpoint 服务端口
	Endpoint() *endpoint.Endpoint
	// RegisterService 注册服务
	RegisterService(desc, service interface{}) error
}

type Client interface {
	// Call 调用服务方法
	Call(ctx context.Context, service, method string, args interface{}, reply interface{}, opts ...interface{}) error
	// Client 获取内部客户端
	Client() interface{}
}

type Transporter interface {
	// Name 获取传输器组件名
	Name() string
	// SetDefaultDiscovery 设置默认的服务发现组件
	SetDefaultDiscovery(discovery registry.Discovery)
	// NewServer 新建传输服务器
	NewServer() (Server, error)
	// NewClient 新建传输务客户端
	NewClient(target string) (Client, error)
}
