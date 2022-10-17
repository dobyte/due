package gate

import (
	"context"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/transport"
	"github.com/dobyte/due/transport/grpc/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type client struct {
	client pb.GateClient
}

func NewClient(ep *router.Endpoint) (*client, error) {
	conn, err := grpc.Dial(ep.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &client{client: pb.NewGateClient(conn)}, nil
}

// Bind 绑定用户与连接
func (c *client) Bind(ctx context.Context, req *transport.BindRequest) (*transport.BindReply, error) {
	_, err := c.client.Bind(ctx, &pb.BindRequest{
		CID: req.CID,
		UID: req.UID,
	})
	if err != nil {
		return nil, err
	}

	return &transport.BindReply{}, nil
}

// Unbind 解绑用户与连接
func (c *client) Unbind(ctx context.Context, req *transport.UnbindRequest) (*transport.UnbindReply, error) {
	_, err := c.client.Unbind(ctx, &pb.UnbindRequest{
		UID: req.UID,
	})
	if err != nil {
		return nil, err
	}

	return &transport.UnbindReply{}, nil
}

// GetIP 获取客户端IP
func (c *client) GetIP(ctx context.Context, nid string, kind int32, target int64) (string, error) {
	reply, err := c.client.GetIP(ctx, &pb.GetIPRequest{
		NID:    nid,
		Kind:   kind,
		Target: target,
	})
	if err != nil {
		return "", err
	}

	return reply.IP, nil
}
