/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/7 1:31 上午
 * @Desc: TODO
 */

package gate

import (
	"context"
	"github.com/dobyte/due/transport/grpc"
	"time"

	"github.com/dobyte/due/network"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/third/redis"
)

type Option func(o *options)

type options struct {
	id       string            // 实例ID
	name     string            // 实例名称
	ctx      context.Context   // 上下文
	redis    redis.Client      // redis客户端
	server   network.Server    // 服务器
	grpc     *grpc.Server      // GRPC服务器
	registry registry.Registry // 服务注册
	timeout  time.Duration     // rpc调用超时时间
}

// WithID 设置实例ID
func WithID(id string) Option { return func(o *options) { o.id = id } }

// WithName 设置实例名称
func WithName(name string) Option { return func(o *options) { o.name = name } }

// WithContext 设置上下文
func WithContext(ctx context.Context) Option { return func(o *options) { o.ctx = ctx } }

// WithRedis 设置redis客户端
func WithRedis(redis redis.Client) Option { return func(o *options) { o.redis = redis } }

// WithServer 设置服务器
func WithServer(s network.Server) Option { return func(o *options) { o.server = s } }

// WithGRPCServer 设置GRPC服务器
func WithGRPCServer(grpc *grpc.Server) Option { return func(o *options) { o.grpc = grpc } }

// WithTimeout 设置RPC调用超时时间
func WithTimeout(timeout time.Duration) Option { return func(o *options) { o.timeout = timeout } }

// WithRegistry 设置服务注册
func WithRegistry(r registry.Registry) Option { return func(o *options) { o.registry = r } }
