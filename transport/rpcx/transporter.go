package rpcx

import (
	"sync"

	"github.com/dobyte/due/transport/rpcx/v2/internal/client"
	"github.com/dobyte/due/transport/rpcx/v2/internal/logger"
	"github.com/dobyte/due/transport/rpcx/v2/internal/server"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
)

const name = "rpcx"

// Transporter 微服务传输器，负责创建服务端与客户端，并管理客户端连接生命周期
type Transporter struct {
	opts    *options
	once    sync.Once
	builder *client.Builder
}

var _ transport.Transporter = &Transporter{}

// NewTransporter 新建传输器
// @param opts ...Option 可选配置项
// @return @1 *Transporter 传输器实例
func NewTransporter(opts ...Option) *Transporter {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	logger.InitLogger()

	return &Transporter{opts: o}
}

// Name 获取传输器组件名
// @return @1 string 组件名
func (t *Transporter) Name() string {
	return name
}

// SetDefaultDiscovery 设置默认的服务发现组件
// 仅当客户端尚未配置服务发现组件时生效
// @param discovery registry.Discovery 服务发现组件
func (t *Transporter) SetDefaultDiscovery(discovery registry.Discovery) {
	if t.opts.client.Discovery == nil {
		t.opts.client.Discovery = discovery
	}
}

// NewServer 新建传输服务器
// @return @1 transport.Server 微服务服务器
// @return @2 error 错误信息
func (t *Transporter) NewServer() (transport.Server, error) {
	return server.NewServer(&t.opts.server)
}

// NewClient 新建传输客户端
// target参数可分为三种模式:
// 服务直连模式: 	direct://127.0.0.1:8011
// 服务直连模式: 	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
// 服务发现模式: 	discovery://service_name
// @param target string 目标服务地址
// @return @1 transport.Client 微服务客户端
// @return @2 error 错误信息
func (t *Transporter) NewClient(target string) (transport.Client, error) {
	t.once.Do(func() {
		t.builder = client.NewBuilder(&t.opts.client)
	})

	cli, err := t.builder.Build(target)
	if err != nil {
		return nil, err
	}

	return client.NewClient(cli), nil
}

// Close 关闭传输器，释放全部客户端连接与资源
// @return @1 error 错误信息
func (t *Transporter) Close() error {
	if t.builder == nil {
		return nil
	}

	return t.builder.Close()
}
