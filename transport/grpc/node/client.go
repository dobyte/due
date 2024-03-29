package node

import (
	"context"
	"github.com/symsimmy/due/transport"
	"github.com/symsimmy/due/transport/grpc/code"
	"github.com/symsimmy/due/transport/grpc/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
)

type Client struct {
	client pb.NodeClient
}

func NewClient(cc *grpc.ClientConn) *Client {
	return &Client{client: pb.NewNodeClient(cc)}
}

// Trigger 触发事件
func (c *Client) Trigger(ctx context.Context, args *transport.TriggerArgs) (miss bool, err error) {
	_, err = c.client.Trigger(ctx, &pb.TriggerRequest{
		Event: int32(args.Event),
		GID:   args.GID,
		CID:   args.CID,
		UID:   args.UID,
	})

	miss = status.Code(err) == code.NotFoundSession

	return
}

// Deliver 投递消息
func (c *Client) Deliver(ctx context.Context, args *transport.DeliverArgs) (miss bool, err error) {
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
