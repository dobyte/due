package client

import (
	"context"
	cli "github.com/smallnest/rpcx/client"
)

type Client struct {
	cli *cli.OneClient
}

func NewClient(cli *cli.OneClient) *Client {
	return &Client{cli: cli}
}

// Call 调用服务方法
func (c *Client) Call(ctx context.Context, service, method string, args interface{}, reply interface{}, opts ...interface{}) error {
	return c.cli.Call(ctx, service, method, args, reply)
}

// Client 获取客户端
func (c *Client) Client() interface{} {
	return c.cli
}
