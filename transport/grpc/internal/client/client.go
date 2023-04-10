package client

import (
	"context"
	"google.golang.org/grpc"
)

type Client struct {
	cc *grpc.ClientConn
}

func NewClient(cc *grpc.ClientConn) *Client {
	return &Client{cc: cc}
}

// Call 调用服务方法
func (c *Client) Call(ctx context.Context, service, method string, args interface{}, reply interface{}, opts ...interface{}) error {
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

// Client 获取GRPC客户端
func (c *Client) Client() interface{} {
	return c.cc
}
