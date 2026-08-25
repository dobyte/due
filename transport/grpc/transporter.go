// Package grpc 基于 gRPC 协议实现微服务传输组件，提供服务端与客户端的创建、连接管理与资源释放能力
package grpc

import (
	"sync"

	"github.com/dobyte/due/transport/grpc/v2/internal/client"
	"github.com/dobyte/due/transport/grpc/v2/internal/server"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
)

const name = "grpc"

// Transporter 微服务传输器，负责创建服务端与客户端，并管理客户端连接生命周期
type Transporter struct {
	opts    *options
	once    sync.Once
	builder *client.Builder
}

// NewTransporter 新建传输器
// @param opts ...Option 可选配置项
// @return @1 *Transporter 传输器实例
func NewTransporter(opts ...Option) *Transporter {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Transporter{opts: o}
}

// Name 获取传输器组件名
func (t *Transporter) Name() string {
	return name
}

// SetDefaultDiscovery 设置默认的服务发现组件
func (t *Transporter) SetDefaultDiscovery(discovery registry.Discovery) {
	if t.opts.client.Discovery == nil {
		t.opts.client.Discovery = discovery
	}
}

// NewServer 新建微服务服务器
func (t *Transporter) NewServer() (transport.Server, error) {
	return server.NewServer(&t.opts.server)
}

// NewClient 新建微服务客户端
// target参数可分为三种模式:
// 服务直连模式: 	direct://127.0.0.1:8011
// 服务直连模式: 	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
// 服务发现模式: 	discovery://service_name
func (t *Transporter) NewClient(target string) (transport.Client, error) {
	t.once.Do(func() {
		t.builder = client.NewBuilder(&t.opts.client)
	})

	cc, err := t.builder.Build(target)
	if err != nil {
		return nil, err
	}

	return client.NewClient(cc), nil
}

// Close 关闭传输器，释放全部客户端连接与资源
// @return @1 error 错误信息
func (t *Transporter) Close() error {
	if t.builder == nil {
		return nil
	}

	return t.builder.Close()
}
