package gate

import (
	"context"

	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/drpc"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

type Server struct {
	*drpc.Server
	provider Provider
}

type ServerOptions = drpc.ServerOptions

func NewServer(provider Provider, opts *ServerOptions) (*Server, error) {
	serv, err := drpc.NewServer(opts)
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
func (s *Server) bind(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, cid, uid, err := protocol.DecodeBindReq(data)
	if err != nil {
		return err
	}

	if err = s.provider.Bind(context.Background(), cid, uid); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeBindRes(seq, codes.ErrorToCode(err)))
	}
}

// 解绑用户
func (s *Server) unbind(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, uid, err := protocol.DecodeUnbindReq(data)
	if err != nil {
		return err
	}

	if err = s.provider.Unbind(context.Background(), uid); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeUnbindRes(seq, codes.ErrorToCode(err)))
	}
}

// 获取IP地址
func (s *Server) getIP(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, kind, target, err := protocol.DecodeGetIPReq(data)
	if err != nil {
		return err
	}

	if ip, err := s.provider.GetIP(context.Background(), kind, target); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeGetIPRes(seq, codes.ErrorToCode(err), ip))
	}
}

// 统计在线人数
func (s *Server) stat(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, kind, err := protocol.DecodeStatReq(data)
	if err != nil {
		return err
	}

	if total, err := s.provider.Stat(context.Background(), kind); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeStatRes(seq, codes.ErrorToCode(err), uint64(total)))
	}
}

// 检测用户是否在线
func (s *Server) isOnline(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, kind, target, err := protocol.DecodeIsOnlineReq(data)
	if err != nil {
		return err
	}

	if isOnline, err := s.provider.IsOnline(context.Background(), kind, target); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeIsOnlineRes(seq, codes.ErrorToCode(err), isOnline))
	}
}

// 断开连接
func (s *Server) disconnect(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, kind, target, force, err := protocol.DecodeDisconnectReq(data)
	if err != nil {
		return err
	}

	if err = s.provider.Disconnect(context.Background(), kind, target, force); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeDisconnectRes(seq, codes.ErrorToCode(err)))
	}
}

// 推送单个消息
func (s *Server) push(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, kind, target, disconnect, message, err := protocol.DecodePushReq(data)
	if err != nil {
		return err
	}

	if err = s.provider.Push(context.Background(), kind, target, disconnect, message); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodePushRes(seq, codes.ErrorToCode(err)))
	}
}

// 推送组播消息
func (s *Server) multicast(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, kind, targets, disconnect, message, err := protocol.DecodeMulticastReq(data)
	if err != nil {
		return err
	}

	if total, err := s.provider.Multicast(context.Background(), kind, targets, disconnect, message); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeMulticastRes(seq, codes.ErrorToCode(err), uint64(total)))
	}
}

// 推送广播消息
func (s *Server) broadcast(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, kind, disconnect, message, err := protocol.DecodeBroadcastReq(data)
	if err != nil {
		return err
	}

	if total, err := s.provider.Broadcast(context.Background(), kind, disconnect, message); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeBroadcastRes(seq, codes.ErrorToCode(err), uint64(total)))
	}
}

// 发布频道消息
func (s *Server) publish(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, channel, disconnect, message, err := protocol.DecodePublishReq(data)
	if err != nil {
		return err
	}

	if total, err := s.provider.Publish(context.Background(), channel, disconnect, message); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodePublishRes(seq, codes.ErrorToCode(err), uint64(total)))
	}
}

// 订阅频道
func (s *Server) subscribe(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, kind, targets, channel, err := protocol.DecodeSubscribeReq(data)
	if err != nil {
		return err
	}

	if err = s.provider.Subscribe(context.Background(), kind, targets, channel); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeSubscribeRes(seq, codes.ErrorToCode(err)))
	}
}

// 取消订阅频道
func (s *Server) unsubscribe(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, kind, targets, channel, err := protocol.DecodeUnsubscribeReq(data)
	if err != nil {
		return err
	}

	if err = s.provider.Unsubscribe(context.Background(), kind, targets, channel); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeUnsubscribeRes(seq, codes.ErrorToCode(err)))
	}
}

// 获取状态
func (s *Server) getState(conn *drpc.ServerConn, seq uint64, data []byte) error {
	if _, err := protocol.DecodeGetStateReq(data); err != nil {
		return err
	}

	if state, err := s.provider.GetState(); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeGetStateRes(seq, codes.ErrorToCode(err), state))
	}
}

// 设置状态
func (s *Server) setState(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, state, err := protocol.DecodeSetStateReq(data)
	if err != nil {
		return err
	}

	if err = s.provider.SetState(state); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeSetStateRes(seq, codes.ErrorToCode(err)))
	}
}
