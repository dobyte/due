package service

import (
	"context"
	cli "github.com/dobyte/due/transport/grpc/internal/client"
	"google.golang.org/grpc"
)

type Client struct {
	cc *grpc.ClientConn
}

func NewClient(target string, opts *cli.Options) (*Client, error) {
	cc, err := cli.Dial(target, opts)
	if err != nil {
		return nil, err
	}

	return &Client{cc: cc}, nil
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

// Conn 获取连接
func (c *Client) Conn() interface{} {
	return c.cc
}
