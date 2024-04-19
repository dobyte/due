package gate

import (
	"context"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/transport/drpc/internal/codes"
	"github.com/dobyte/due/v2/transport/drpc/internal/packet"
	"github.com/dobyte/due/v2/transport/drpc/internal/route"
	"github.com/dobyte/due/v2/transport/drpc/internal/server"
	"time"
)

func NewServer(provider transport.GateProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	e := &endpoint{
		provider:         provider,
		bindPacker:       packet.NewBindPacker(),
		unbindPacker:     packet.NewUnbindPacker(),
		getIPPacker:      packet.NewGetIPPacker(),
		statPacker:       packet.NewStatPacker(),
		disconnectPacker: packet.NewDisconnectPacker(),
		pushPacker:       packet.NewPushPacker(),
	}

	e.init(s)

	return s, nil
}

type endpoint struct {
	provider         transport.GateProvider
	bindPacker       *packet.BindPacker
	unbindPacker     *packet.UnbindPacker
	getIPPacker      *packet.GetIPPacker
	statPacker       *packet.StatPacker
	disconnectPacker *packet.DisconnectPacker
	pushPacker       *packet.PushPacker
}

func (e *endpoint) init(s *server.Server) {
	// 注册绑定路由处理器
	s.RegisterHandler(route.Bind, e.bind)
	// 注册推送路由处理器
	s.RegisterHandler(route.Push, e.push)
}

func (e *endpoint) bind(conn *server.Conn, data []byte) error {
	seq, cid, uid, err := e.bindPacker.UnpackReq(data)
	if err != nil {
		return err
	}

	var code int16

	if err = e.provider.Bind(context.Background(), cid, uid); err != nil {
		switch {
		case errors.Is(err, errors.ErrNotFoundSession):
			code = codes.NotFoundSession
		default:
			code = codes.NotFoundSession
		}
	}

	buf, err := e.bindPacker.PackRes(seq, code)
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)

	return conn.Send(buf)
}

func (e *endpoint) push(conn *server.Conn, data []byte) error {
	seq, kind, target, message, err := e.pushPacker.UnpackReq(data)
	if err != nil {
		return err
	}

	//fmt.Println(seq, kind, target, message.Seq, message.Route, string(message.Buffer))

	var code int16

	if err = e.provider.Push(context.Background(), kind, target, message); err != nil {
		switch {
		case errors.Is(err, errors.ErrNotFoundSession):
			code = codes.NotFoundSession
		default:
			code = codes.NotFoundSession
		}
	}

	buf, err := e.pushPacker.PackRes(seq, code)
	if err != nil {
		return err
	}

	return conn.Send(buf)
}
