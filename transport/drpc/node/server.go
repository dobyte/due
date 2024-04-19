package node

import (
	"context"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/transport/drpc/internal/codes"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"github.com/dobyte/due/v2/transport/drpc/internal/route"
	"github.com/dobyte/due/v2/transport/drpc/internal/server"
)

func NewServer(provider transport.NodeProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	e := &endpoint{
		provider:      provider,
		deliverPacker: packet.NewDeliverPacker(),
	}

	e.init(s)

	return s, nil
}

type endpoint struct {
	provider      transport.NodeProvider
	deliverPacker *packet.DeliverPacker
}

func (e *endpoint) init(s *server.Server) {
	// 注册投递路由处理器
	s.RegisterHandler(route.Deliver, e.deliver)
}

func (e *endpoint) deliver(conn *server.Conn, data []byte) error {
	seq, gid, cid, uid, message, err := e.deliverPacker.UnpackReq(data)
	if err != nil {
		return err
	}

	var code int16

	miss, err := e.provider.Deliver(context.Background(), &transport.DeliverArgs{
		GID:     gid,
		CID:     cid,
		UID:     uid,
		Message: message,
	})
	if err != nil {
		code = codes.Internal
	} else {
		if miss {
			code = codes.NotFoundSession
		}
	}

	buf, err := e.deliverPacker.PackRes(seq, code)
	if err != nil {
		return err
	}

	return conn.Send(buf)
}
