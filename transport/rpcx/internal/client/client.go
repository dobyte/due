package client

import (
	"context"

	cli "github.com/smallnest/rpcx/client"
)

// Client 微服务客户端，封装 rpcx 客户端并提供通用方法调用能力
type Client struct {
	cli *cli.OneClient
}

// NewClient 新建微服务客户端
// @param cli *cli.OneClient rpcx 客户端实例
// @return @1 *Client 客户端实例
func NewClient(cli *cli.OneClient) *Client {
	return &Client{cli: cli}
}

// Call 调用服务方法
// @param ctx context.Context 上下文
// @param service string 服务名
// @param method string 方法名
// @param args any 请求参数
// @param reply any 响应参数
// @param opts ...any 调用选项
// @return @1 error 错误信息
func (c *Client) Call(ctx context.Context, service, method string, args any, reply any, opts ...any) error {
	return c.cli.Call(ctx, service, method, args, reply)
}

// Client 获取客户端
// @return @1 any 原始 rpcx 客户端实例
func (c *Client) Client() any {
	return c.cli
}
