package node

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/grpc/internal/code"
	"github.com/dobyte/due/transport/grpc/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type client struct {
	client pb.NodeClient
}

func NewClient(ep *router.Endpoint) (*client, error) {
	conn, err := grpc.Dial(ep.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &client{client: pb.NewNodeClient(conn)}, nil
}

// Trigger 触发事件
func (c *client) Trigger(ctx context.Context, event cluster.Event, gid string, uid int64) (miss bool, err error) {
	_, err = c.client.Trigger(ctx, &pb.TriggerRequest{
		Event: int32(event),
		GID:   gid,
		UID:   uid,
	})

	miss = status.Code(err) == code.NotFoundSession

	return
}

// Deliver 投递消息
func (c *client) Deliver(ctx context.Context, gid, nid string, cid, uid int64, message *transport.Message) (miss bool, err error) {
	_, err = c.client.Deliver(ctx, &pb.DeliverRequest{
		GID: gid,
		NID: nid,
		CID: cid,
		UID: uid,
		Message: &pb.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		},
	})

	miss = status.Code(err) == code.NotFoundSession

	return
}
