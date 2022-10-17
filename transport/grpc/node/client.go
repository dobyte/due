package node

import (
	"context"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/grpc/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	client pb.NodeClient
}

func NewClient(ep *router.Endpoint) (*Client, error) {
	conn, err := grpc.Dial(ep.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{client: pb.NewNodeClient(conn)}, nil
}

// Trigger 触发事件
func (c *Client) Trigger(ctx context.Context, req *transport.TriggerRequest) (*transport.TriggerReply, error) {
	_, err := c.client.Trigger(ctx, &pb.TriggerRequest{
		Event: req.Event,
		GID:   req.GID,
		UID:   req.UID,
	})
	if err != nil {
		return nil, err
	}

	return &transport.TriggerReply{}, nil
}

// Deliver 投递消息
func (c *Client) Deliver(ctx context.Context, req *transport.DeliverRequest) (*transport.DeliverReply, error) {
	_, err := c.client.Deliver(ctx, &pb.DeliverRequest{
		GID: req.GID,
		NID: req.NID,
		CID: req.CID,
		UID: req.UID,
		Message: &pb.Message{
			Seq:    req.Message.Seq,
			Route:  req.Message.Route,
			Buffer: req.Message.Buffer,
		},
	})
	if err != nil {
		return nil, err
	}

	return nil, err
}
