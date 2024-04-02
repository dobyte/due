package gate

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/symsimmy/due/errcode"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/packet"
	"github.com/symsimmy/due/session"
	"github.com/symsimmy/due/transport"
	"github.com/symsimmy/due/transport/gnet/internal/pb"
	"github.com/symsimmy/due/transport/gnet/internal/server"
)

type endpoint struct {
	provider transport.GateProvider
}

func NewServer(provider transport.GateProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}
	endpoint := endpoint{provider: provider}
	s.OnReceive(endpoint.dispatch)
	return s, nil
}

func (e *endpoint) dispatch(methodName uint16, data []byte) (reply interface{}, err error) {
	switch methodName {
	case transport.Ping:
		req := &pb.PingRequest{}
		err = proto.Unmarshal(data, req)
		if err != nil {
			return &pb.PingReply{
				ErrorCode:    errcode.Invalid_pb_message,
				ErrorMessage: err.Error(),
			}, err
		}
		reply = &pb.PingReply{
			Reply: req.Message,
		}
		break
	case transport.Bind:
		req := &pb.BindRequest{}
		err = proto.Unmarshal(data, req)
		if err != nil {
			return &pb.BindReply{
				ErrorCode:    errcode.Invalid_pb_message,
				ErrorMessage: err.Error(),
			}, err
		}
		go func(e *endpoint) {
			_ = e.provider.Bind(context.Background(), req.CID, req.UID)
		}(e)
		reply = &pb.BindReply{}
		break
	case transport.Unbind:
		req := &pb.UnbindRequest{}
		err = proto.Unmarshal(data, req)
		if err != nil {
			return nil, err
		}
		go func(e *endpoint) {
			_ = e.provider.Unbind(context.Background(), req.UID)
		}(e)
		reply = &pb.UnbindReply{}
		break
	case transport.Disconnect:
		req := &pb.DisconnectRequest{}
		err = proto.Unmarshal(data, req)
		if err != nil {
			return &pb.DisconnectReply{
				ErrorCode:    errcode.Invalid_pb_message,
				ErrorMessage: err.Error(),
			}, err
		}
		err = e.provider.Disconnect(context.Background(), session.Kind(req.Kind), req.Target, req.IsForce)
		if err != nil {
			return &pb.DisconnectReply{
				ErrorCode:    errcode.Internal_server_error,
				ErrorMessage: err.Error(),
			}, err
		}
		reply = &pb.DisconnectReply{}
		break
	case transport.GetID:
		req := &pb.GetIdRequest{}
		err = proto.Unmarshal(data, req)
		if err != nil {
			return &pb.GetIdReply{
				ErrorCode:    errcode.Invalid_pb_message,
				ErrorMessage: err.Error(),
			}, err
		}
		var id int64
		id, err = e.provider.GetID(context.Background(), session.Kind(req.Kind), req.Target)
		if err != nil {
			return &pb.GetIdReply{
				Id:           0,
				ErrorCode:    errcode.Internal_server_error,
				ErrorMessage: err.Error(),
			}, err
		}
		reply = &pb.GetIdReply{Id: id}
		break
	case transport.GetIP:
		req := &pb.GetIPRequest{}
		err = proto.Unmarshal(data, req)
		if err != nil {
			return &pb.GetIPReply{
				ErrorCode:    errcode.Invalid_pb_message,
				ErrorMessage: err.Error(),
			}, err
		}
		var ip string
		ip, err := e.provider.GetIP(context.Background(), session.Kind(req.Kind), req.Target)
		if err != nil {
			return &pb.GetIPReply{
				ErrorCode:    errcode.Internal_server_error,
				ErrorMessage: err.Error(),
			}, err
		}
		reply = &pb.GetIPReply{IP: ip}
		break
	case transport.Stat:
		req := &pb.StatRequest{}
		err := proto.Unmarshal(data, req)
		if err != nil {
			return &pb.StatReply{
				ErrorCode:    errcode.Invalid_pb_message,
				ErrorMessage: err.Error(),
			}, err
		}
		total, err := e.provider.Stat(context.Background(), session.Kind(req.Kind))
		if err != nil {
			return &pb.StatReply{
				ErrorCode:    errcode.Internal_server_error,
				ErrorMessage: err.Error(),
			}, err
		}
		reply = &pb.StatReply{Total: total}
		break
	case transport.IsOnline:
		req := &pb.IsOnlineRequest{}
		err = proto.Unmarshal(data, req)
		if err != nil {
			return &pb.IsOnlineReply{
				ErrorCode:    errcode.Invalid_pb_message,
				ErrorMessage: err.Error(),
			}, err
		}
		var isOnline bool
		isOnline, err = e.provider.IsOnline(context.Background(), session.Kind(req.Kind), req.Target)
		if err != nil {
			return &pb.IsOnlineReply{
				ErrorCode:    errcode.Internal_server_error,
				ErrorMessage: err.Error(),
			}, err
		}
		reply = &pb.IsOnlineReply{IsOnline: isOnline}
		break
	case transport.Push:
		req := &pb.PushRequest{}
		err = proto.Unmarshal(data, req)
		if err != nil {
			return nil, err
		}
		err = e.provider.Push(context.Background(), session.Kind(req.Kind), req.Target, &packet.Message{
			Seq:    req.Message.Seq,
			Route:  req.Message.Route,
			Buffer: req.Message.Buffer,
		})

		if err != nil {
			return nil, err
		}

		break
	case transport.Multicast:
		req := &pb.MulticastRequest{}
		err := proto.Unmarshal(data, req)
		if err != nil {
			return nil, err
		}
		_, err = e.provider.Multicast(context.Background(), session.Kind(req.Kind), req.Targets, &packet.Message{
			Seq:    req.Message.Seq,
			Route:  req.Message.Route,
			Buffer: req.Message.Buffer,
		})

		if err != nil {
			return nil, err
		}

		break
	case transport.Broadcast:
		req := &pb.BroadcastRequest{}
		err = proto.Unmarshal(data, req)
		if err != nil {
			return
		}
		_, err = e.provider.Broadcast(context.Background(), session.Kind(req.Kind), &packet.Message{
			Seq:    req.Message.Seq,
			Route:  req.Message.Route,
			Buffer: req.Message.Buffer,
		})

		if err != nil {
			return nil, err
		}

		break
	case transport.BlockConn:
		req := &pb.BlockConnRequest{}
		err = proto.Unmarshal(data, req)
		if err != nil {
			return nil, err
		}
		e.provider.Block(context.Background(), req.ONid, req.NNid, req.Target)

		break
	case transport.ReleaseConn:
		req := &pb.ReleaseConnRequest{}
		err = proto.Unmarshal(data, req)
		if err != nil {
			return nil, err
		}
		e.provider.Release(context.Background(), req.Target)

		break
	}

	if err != nil {
		log.Warnf("gate server dispatch methodName:%+v failed.error:%+v", methodName, err)
	}
	return
}
