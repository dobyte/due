/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/7 1:31 上午
 * @Desc: TODO
 */

package gate

import (
	"context"
	"github.com/dobyte/due/locate"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/grpc"
	"time"

	"github.com/dobyte/due/network"
	"github.com/dobyte/due/registry"
)

type Option func(o *options)

type options struct {
	id          string                // 实例ID
	name        string                // 实例名称
	ctx         context.Context       // 上下文
	server      network.Server        // 服务器
	grpc        *grpc.Server          // GRPC服务器
	timeout     time.Duration         // rpc调用超时时间
	locator     locate.Locator        // 定位器
	registry    registry.Registry     // 服务注册
	transporter transport.Transporter // 传输器
}

// WithID 设置实例ID
func WithID(id string) Option {
	return func(o *options) { o.id = id }
}

// WithName 设置实例名称
func WithName(name string) Option {
	return func(o *options) { o.name = name }
}

// WithContext 设置上下文
func WithContext(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// WithServer 设置服务器
func WithServer(server network.Server) Option {
	return func(o *options) { o.server = server }
}

// WithTimeout 设置RPC调用超时时间
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
}

// WithLocator 设置定位器
func WithLocator(locator locate.Locator) Option {
	return func(o *options) { o.locator = locator }
}

// WithRegistry 设置服务注册
func WithRegistry(r registry.Registry) Option {
	return func(o *options) { o.registry = r }
}

// WithTransporter 设置传输器
func WithTransporter(transporter *grpc.Server) Option {
	return func(o *options) { o.transporter = transporter }
}
