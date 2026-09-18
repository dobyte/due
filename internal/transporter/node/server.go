package node

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
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
func (s *Server) trigger(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	event, cid, uid, err := protocol.DecodeTriggerReq(req)

	req.Release()

	if err != nil {
		return err
	}

	kind, inst := conn.HandshakeInfo()

	if kind != cluster.Gate {
		return errors.ErrIllegalRequest
	}

	err = s.provider.Trigger(context.Background(), inst, cid, uid, event)

	if seq == 0 {
		if errors.Is(err, errors.ErrNotFoundSession) {
			return nil
		} else {
			return err
		}
	} else {
		return conn.Push(protocol.EncodeTriggerRes(seq, codes.ErrorToCode(err)))
	}
}

// 投递消息
func (s *Server) deliver(conn *drpc.ServerConn, seq uint64, req *buffer.Bytes) error {
	var (
		gid string
		nid string
	)

	switch kind, inst := conn.HandshakeInfo(); kind {
	case cluster.Gate:
		gid = inst
	case cluster.Node:
		nid = inst
	default:
		req.Release()
		return errors.ErrIllegalRequest
	}

	cid, uid, buf, err := protocol.DecodeDeliverReq(req)
	if err != nil {
		req.Release()
		return err
	}

	err = s.provider.Deliver(context.Background(), gid, nid, cid, uid, buf)

	if seq == 0 {
		return err
	} else {
		return conn.Push(protocol.EncodeDeliverRes(seq, codes.ErrorToCode(err)))
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
