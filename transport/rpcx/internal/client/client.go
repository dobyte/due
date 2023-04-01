package client

import (
	"context"
	ep "github.com/dobyte/due/internal/endpoint"
	cli "github.com/smallnest/rpcx/client"
	proto "github.com/smallnest/rpcx/protocol"
)

type Client struct {
	discovery cli.ServiceDiscovery
	option    cli.Option
}

func NewClient(ep *ep.Endpoint) (*Client, error) {
	discovery, err := cli.NewPeer2PeerDiscovery("tcp@"+ep.Address(), "")
	if err != nil {
		return nil, err
	}

	c := &Client{}
	c.discovery = discovery
	c.option = cli.DefaultOption
	c.option.CompressType = proto.Gzip

	return c, nil
}

// Call 调用服务
func (c *Client) Call(ctx context.Context, service, method string, args, reply interface{}) error {
	cc := cli.NewXClient(service, cli.Failtry, cli.RandomSelect, c.discovery, c.option)

	return cc.Call(ctx, method, args, reply)
}
