package gate

import (
	"context"

	"github.com/dobyte/due/v2/core/buffer"
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
func (s *Server) bind(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	cid, uid, err := protocol.DecodeBindReq(req)

	req.Release()

	if err != nil {
		return err
	}

	err = s.provider.Bind(context.Background(), cid, uid)

	if seq == 0 {
		return err
	}

	return conn.Push(protocol.EncodeBindRes(seq, codes.ErrorToCode(err)))
}

// 解绑用户
func (s *Server) unbind(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	uid, err := protocol.DecodeUnbindReq(req)

	req.Release()

	if err != nil {
		return err
	}

	err = s.provider.Unbind(context.Background(), uid)

	if seq == 0 {
		return err
	}

	return conn.Push(protocol.EncodeUnbindRes(seq, codes.ErrorToCode(err)))
}

// 获取IP地址
func (s *Server) getIP(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	kind, target, err := protocol.DecodeGetIPReq(req)

	req.Release()

	if err != nil {
		return err
	}

	ip, err := s.provider.GetIP(context.Background(), kind, target)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodeGetIPRes(seq, codes.ErrorToCode(err), ip))
	}
}

// 统计在线人数
func (s *Server) stat(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	kind, err := protocol.DecodeStatReq(req)

	req.Release()

	if err != nil {
		return err
	}

	total, err := s.provider.Stat(context.Background(), kind)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodeStatRes(seq, codes.ErrorToCode(err), uint64(total)))
	}
}

// 检测用户是否在线
func (s *Server) isOnline(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	kind, target, err := protocol.DecodeIsOnlineReq(req)

	req.Release()

	if err != nil {
		return err
	}

	isOnline, err := s.provider.IsOnline(context.Background(), kind, target)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodeIsOnlineRes(seq, codes.ErrorToCode(err), isOnline))
	}
}

// 断开连接
func (s *Server) disconnect(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	kind, target, force, err := protocol.DecodeDisconnectReq(req)

	req.Release()

	if err != nil {
		return err
	}

	err = s.provider.Disconnect(context.Background(), kind, target, force)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodeDisconnectRes(seq, codes.ErrorToCode(err)))
	}
}

// 推送单个消息
// 注意：buf不进行释放，需要在消息发送时进行释放
func (s *Server) push(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	kind, target, disconnect, buf, err := protocol.DecodePushReq(req)
	if err != nil {
		req.Release()
		return err
	}

	err = s.provider.Push(context.Background(), kind, target, disconnect, buf)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodePushRes(seq, codes.ErrorToCode(err)))
	}
}

// 推送组播消息
func (s *Server) multicast(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	kind, targets, disconnect, buf, err := protocol.DecodeMulticastReq(req)
	if err != nil {
		req.Release()
		return err
	}

	total, err := s.provider.Multicast(context.Background(), kind, targets, disconnect, buf)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodeMulticastRes(seq, codes.ErrorToCode(err), uint64(total)))
	}
}

// 推送广播消息
func (s *Server) broadcast(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	kind, disconnect, buf, err := protocol.DecodeBroadcastReq(req)
	if err != nil {
		req.Release()
		return err
	}

	total, err := s.provider.Broadcast(context.Background(), kind, disconnect, buf)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodeBroadcastRes(seq, codes.ErrorToCode(err), uint64(total)))
	}
}

// 发布频道消息
func (s *Server) publish(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	channel, disconnect, buf, err := protocol.DecodePublishReq(req)
	if err != nil {
		req.Release()
		return err
	}

	total, err := s.provider.Publish(context.Background(), channel, disconnect, buf)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodePublishRes(seq, codes.ErrorToCode(err), uint64(total)))
	}
}

// 订阅频道
func (s *Server) subscribe(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	kind, targets, channel, err := protocol.DecodeSubscribeReq(req)

	req.Release()

	if err != nil {
		return err
	}

	err = s.provider.Subscribe(context.Background(), kind, targets, channel)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodeSubscribeRes(seq, codes.ErrorToCode(err)))
	}
}

// 取消订阅频道
func (s *Server) unsubscribe(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	kind, targets, channel, err := protocol.DecodeUnsubscribeReq(req)

	req.Release()

	if err != nil {
		return err
	}

	err = s.provider.Unsubscribe(context.Background(), kind, targets, channel)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodeUnsubscribeRes(seq, codes.ErrorToCode(err)))
	}
}

// 获取状态
func (s *Server) getState(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	err := protocol.DecodeGetStateReq(req)

	req.Release()

	if err != nil {
		return err
	}

	state, err := s.provider.GetState()

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodeGetStateRes(seq, codes.ErrorToCode(err), state))
	}
}

// 设置状态
func (s *Server) setState(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	state, err := protocol.DecodeSetStateReq(req)

	req.Release()

	if err != nil {
		return err
	}

	err = s.provider.SetState(state)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodeSetStateRes(seq, codes.ErrorToCode(err)))
	}
}
