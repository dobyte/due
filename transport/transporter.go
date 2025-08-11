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
	RegisterService(desc, service any) error
}

type Client interface {
	// Call 调用服务方法
	Call(ctx context.Context, service, method string, args any, reply any, opts ...any) error
	// Client 获取内部客户端
	Client() any
}

type Transporter interface {
	// Name 获取传输器组件名
	Name() string
	// NewServer 新建传输服务器
	NewServer() (Server, error)
	// NewClient 新建传输务客户端
	// target参数可分为三种模式:
	// 服务直连模式: 	direct://127.0.0.1:8011
	// 服务直连模式: 	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
	// 服务发现模式: 	discovery://service_name
	NewClient(target string) (Client, error)
	// SetDefaultDiscovery 设置默认的服务发现组件
	SetDefaultDiscovery(discovery registry.Discovery)
}

type NewMeshClient func(target string) (Client, error)
