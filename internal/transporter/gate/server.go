package gate

import (
	"context"

	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
	"github.com/dobyte/due/v2/internal/transporter/internal/server"
)

type Server struct {
	*server.Server
	provider Provider
}

type ServerOptions = server.Options

func NewServer(provider Provider, opts *ServerOptions) (*Server, error) {
	serv, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	s := &Server{Server: serv, provider: provider}
	s.init()

	return s, nil
}

func (s *Server) init() {
	s.RegisterHandler(route.Bind, s.bind)
	s.RegisterHandler(route.Unbind, s.unbind)
	s.RegisterHandler(route.GetIP, s.getIP)
	s.RegisterHandler(route.Stat, s.stat)
	s.RegisterHandler(route.IsOnline, s.isOnline)
	s.RegisterHandler(route.Disconnect, s.disconnect)
	s.RegisterHandler(route.Push, s.push)
	s.RegisterHandler(route.Multicast, s.multicast)
	s.RegisterHandler(route.Broadcast, s.broadcast)
	s.RegisterHandler(route.Publish, s.publish)
	s.RegisterHandler(route.Subscribe, s.subscribe)
	s.RegisterHandler(route.Unsubscribe, s.unsubscribe)
	s.RegisterHandler(route.GetState, s.getState)
	s.RegisterHandler(route.SetState, s.setState)
}

// 绑定用户
func (s *Server) bind(conn *server.Conn, data []byte) error {
	seq, cid, uid, err := protocol.DecodeBindReq(data)
	if err != nil {
		return err
	}

	err = s.provider.Bind(context.Background(), cid, uid)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeBindRes(seq, codes.ErrorToCode(err)))
	}
}

// 解绑用户
func (s *Server) unbind(conn *server.Conn, data []byte) error {
	seq, uid, err := protocol.DecodeUnbindReq(data)
	if err != nil {
		return err
	}

	err = s.provider.Unbind(context.Background(), uid)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeUnbindRes(seq, codes.ErrorToCode(err)))
	}
}

// 获取IP地址
func (s *Server) getIP(conn *server.Conn, data []byte) error {
	seq, kind, target, err := protocol.DecodeGetIPReq(data)
	if err != nil {
		return err
	}

	ip, err := s.provider.GetIP(context.Background(), kind, target)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeGetIPRes(seq, codes.ErrorToCode(err), ip))
	}
}

// 统计在线人数
func (s *Server) stat(conn *server.Conn, data []byte) error {
	seq, kind, err := protocol.DecodeStatReq(data)
	if err != nil {
		return err
	}

	total, err := s.provider.Stat(context.Background(), kind)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeStatRes(seq, codes.ErrorToCode(err), uint64(total)))
	}
}

// 检测用户是否在线
func (s *Server) isOnline(conn *server.Conn, data []byte) error {
	seq, kind, target, err := protocol.DecodeIsOnlineReq(data)
	if err != nil {
		return err
	}

	isOnline, err := s.provider.IsOnline(context.Background(), kind, target)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeIsOnlineRes(seq, codes.ErrorToCode(err), isOnline))
	}
}

// 断开连接
func (s *Server) disconnect(conn *server.Conn, data []byte) error {
	seq, kind, target, force, err := protocol.DecodeDisconnectReq(data)
	if err != nil {
		return err
	}

	err = s.provider.Disconnect(context.Background(), kind, target, force)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeDisconnectRes(seq, codes.ErrorToCode(err)))
	}
}

// 推送单个消息
func (s *Server) push(conn *server.Conn, data []byte) error {
	seq, kind, target, message, err := protocol.DecodePushReq(data)
	if err != nil {
		return err
	}

	err = s.provider.Push(context.Background(), kind, target, message)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodePushRes(seq, codes.ErrorToCode(err)))
	}
}

// 推送组播消息
func (s *Server) multicast(conn *server.Conn, data []byte) error {
	seq, kind, targets, message, err := protocol.DecodeMulticastReq(data)
	if err != nil {
		return err
	}

	total, err := s.provider.Multicast(context.Background(), kind, targets, message)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeMulticastRes(seq, codes.ErrorToCode(err), uint64(total)))
	}
}

// 推送广播消息
func (s *Server) broadcast(conn *server.Conn, data []byte) error {
	seq, kind, message, err := protocol.DecodeBroadcastReq(data)
	if err != nil {
		return err
	}

	total, err := s.provider.Broadcast(context.Background(), kind, message)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeBroadcastRes(seq, codes.ErrorToCode(err), uint64(total)))
	}
}

// 发布频道消息
func (s *Server) publish(conn *server.Conn, data []byte) error {
	seq, channel, message, err := protocol.DecodePublishReq(data)
	if err != nil {
		return err
	}

	total := s.provider.Publish(context.Background(), channel, message)

	if seq == 0 {
		return nil
	} else {
		return conn.Send(protocol.EncodePublishRes(seq, uint64(total)))
	}
}

// 订阅频道
func (s *Server) subscribe(conn *server.Conn, data []byte) error {
	seq, kind, targets, channel, err := protocol.DecodeSubscribeReq(data)
	if err != nil {
		return err
	}

	err = s.provider.Subscribe(context.Background(), kind, targets, channel)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeSubscribeRes(seq, codes.ErrorToCode(err)))
	}
}

// 取消订阅频道
func (s *Server) unsubscribe(conn *server.Conn, data []byte) error {
	seq, kind, targets, channel, err := protocol.DecodeUnsubscribeReq(data)
	if err != nil {
		return err
	}

	err = s.provider.Unsubscribe(context.Background(), kind, targets, channel)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeUnsubscribeRes(seq, codes.ErrorToCode(err)))
	}
}

// 获取状态
func (s *Server) getState(conn *server.Conn, data []byte) error {
	seq, err := protocol.DecodeGetStateReq(data)
	if err != nil {
		return err
	}

	state, err := s.provider.GetState()

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeGetStateRes(seq, codes.ErrorToCode(err), state))
	}
}

// 设置状态
func (s *Server) setState(conn *server.Conn, data []byte) error {
	seq, state, err := protocol.DecodeSetStateReq(data)
	if err != nil {
		return err
	}

	err = s.provider.SetState(state)

	if seq == 0 {
		return err
	} else {
		return conn.Send(protocol.EncodeSetStateRes(seq, codes.ErrorToCode(err)))
	}
}
