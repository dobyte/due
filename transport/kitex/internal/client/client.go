package client

import (
	"context"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/callopt"
)

type Client struct {
	cli client.Client
}

func NewClient(cli client.Client) *Client {
	return &Client{cli: cli}
}

// Call 调用服务方法
func (c *Client) Call(ctx context.Context, service, method string, args interface{}, reply interface{}, opts ...interface{}) error {
	options := make([]callopt.Option, 0, len(opts))
	for _, opt := range opts {
		if o, ok := opt.(callopt.Option); ok {
			options = append(options, o)
		}
	}

	return c.cli.Call(client.NewCtxWithCallOptions(ctx, options), method, args, reply)
}

// Client 获取客户端
func (c *Client) Client() interface{} {
	return c.cli
}
