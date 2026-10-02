package client

import (
	"context"

	"google.golang.org/grpc"
)

// Client is a microservice client that wraps a gRPC connection and provides generic method
// invocation.
type Client struct {
	cc *grpc.ClientConn
}

// NewClient returns a new microservice client backed by the given gRPC client connection.
func NewClient(cc *grpc.ClientConn) *Client {
	return &Client{cc: cc}
}

// Call invokes a service method.
//
// It builds the full invocation path from service and method, and passes through any
// grpc.CallOption.
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

// Client returns the underlying gRPC client connection.
func (c *Client) Client() any {
	return c.cc
}
