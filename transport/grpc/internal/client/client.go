package client

import (
	"context"

	"google.golang.org/grpc"
)

// Client 微服务客户端，封装 gRPC 连接并提供通用方法调用能力
type Client struct {
	cc *grpc.ClientConn
}

// NewClient 新建微服务客户端
// @param cc *grpc.ClientConn gRPC 客户端连接
// @return @1 *Client 客户端实例
func NewClient(cc *grpc.ClientConn) *Client {
	return &Client{cc: cc}
}

// Call 调用服务方法
// 通过 service 与 method 拼接完整的调用路径，支持透传任意 grpc.CallOption
// @param ctx context.Context 上下文
// @param service string 服务名
// @param method string 方法名
// @param args any 请求参数
// @param reply any 响应参数
// @param opts ...any 调用选项，仅接受 grpc.CallOption
// @return @1 error 错误信息
func (c *Client) Call(ctx context.Context, service, method string, args any, reply any, opts ...any) error {
	path := ""

	if service != "" {
		path += "/" + service
	}

	if method != "" {
		path += "/" + method
	}

	options := make([]grpc.CallOption, 0, len(opts))
	for _, opt := range opts {
		if o, ok := opt.(grpc.CallOption); ok {
			options = append(options, o)
		}
	}

	return c.cc.Invoke(ctx, path, args, reply, options...)
}

// Client 获取GRPC客户端连接
// @return @1 any 原始 gRPC 客户端连接
func (c *Client) Client() any {
	return c.cc
}
