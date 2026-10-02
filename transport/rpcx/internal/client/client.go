package client

import (
	"context"

	cli "github.com/smallnest/rpcx/client"
)

// Client is a microservice client that wraps an rpcx client and provides generic method invocation.
type Client struct {
	cli *cli.OneClient
}

// NewClient returns a new microservice client backed by the given rpcx client.
func NewClient(cli *cli.OneClient) *Client {
	return &Client{cli: cli}
}

// Call invokes a service method.
func (c *Client) Call(ctx context.Context, service, method string, args any, reply any, opts ...any) error {
	return c.cli.Call(ctx, service, method, args, reply)
}

// Client returns the underlying rpcx client.
func (c *Client) Client() any {
	return c.cli
}
