package node

import (
	"context"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/transport"
	innerclient "github.com/dobyte/due/transport/grpc/internal/client"
	"github.com/dobyte/due/transport/grpc/internal/code"
	"github.com/dobyte/due/transport/grpc/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
	"sync"
)

var clients sync.Map

type client struct {
	client pb.NodeClient
}

func NewClient(ep *router.Endpoint, opts *innerclient.Options) (*client, error) {
	cli, ok := clients.Load(ep.Address())
	if ok {
		return cli.(*client), nil
	}

	opts.Addr = ep.Address()
	opts.IsSecure = ep.IsSecure()

	conn, err := innerclient.Dial(opts)
	if err != nil {
		return nil, err
	}

	cc := &client{client: pb.NewNodeClient(conn)}
	clients.Store(ep.Address(), cc)

	return cc, nil
}

// Trigger 触发事件
func (c *client) Trigger(ctx context.Context, args *transport.TriggerArgs) (miss bool, err error) {
	_, err = c.client.Trigger(ctx, &pb.TriggerRequest{
		Event: int32(args.Event),
		GID:   args.GID,
		UID:   args.UID,
	})

	miss = status.Code(err) == code.NotFoundSession

	return
}

// Deliver 投递消息
func (c *client) Deliver(ctx context.Context, args *transport.DeliverArgs) (miss bool, err error) {
	_, err = c.client.Deliver(ctx, &pb.DeliverRequest{
		GID: args.GID,
		NID: args.NID,
		CID: args.CID,
		UID: args.UID,
		Message: &pb.Message{
			Seq:    args.Message.Seq,
			Route:  args.Message.Route,
			Buffer: args.Message.Buffer,
		},
	}, grpc.UseCompressor(gzip.Name))

	miss = status.Code(err) == code.NotFoundSession

	return
}
