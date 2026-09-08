package node

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
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
	s.RegisterHandler(route.Trigger, s.trigger)
	s.RegisterHandler(route.Deliver, s.deliver)
	s.RegisterHandler(route.GetState, s.getState)
	s.RegisterHandler(route.SetState, s.setState)
}

// 触发事件
func (s *Server) trigger(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, event, cid, uid, err := protocol.DecodeTriggerReq(data)
	if err != nil {
		return err
	}

	if conn.HandshakeInsKind() != cluster.Gate {
		return errors.ErrIllegalRequest
	}

	if err = s.provider.Trigger(context.Background(), conn.HandshakeInsID(), cid, uid, event); seq == 0 {
		if errors.Is(err, errors.ErrNotFoundSession) {
			return nil
		} else {
			return err
		}
	} else {
		return conn.Reply(seq, protocol.EncodeTriggerRes(seq, codes.ErrorToCode(err)))
	}
}

// 投递消息
func (s *Server) deliver(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, cid, uid, message, err := protocol.DecodeDeliverReq(data)
	if err != nil {
		return err
	}

	var (
		gid string
		nid string
	)

	switch conn.HandshakeInsKind() {
	case cluster.Gate:
		gid = conn.HandshakeInsID()
	case cluster.Node:
		nid = conn.HandshakeInsID()
	default:
		return errors.ErrIllegalRequest
	}

	if err = s.provider.Deliver(context.Background(), gid, nid, cid, uid, message); seq == 0 {
		return err
	} else {
		return conn.Reply(seq, protocol.EncodeDeliverRes(seq, codes.ErrorToCode(err)))
	}
}

// 获取状态
func (s *Server) getState(conn *drpc.ServerConn, seq uint64, data []byte) error {
	if _, err := protocol.DecodeGetStateReq(data); err != nil {
		return err
	}

	state, err := s.provider.GetState()

	return conn.Reply(seq, protocol.EncodeGetStateRes(seq, codes.ErrorToCode(err), state))
}

// 设置状态
func (s *Server) setState(conn *drpc.ServerConn, seq uint64, data []byte) error {
	_, state, err := protocol.DecodeSetStateReq(data)
	if err != nil {
		return err
	}

	err = s.provider.SetState(state)

	return conn.Reply(seq, protocol.EncodeSetStateRes(seq, codes.ErrorToCode(err)))
}
