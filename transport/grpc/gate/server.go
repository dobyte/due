package gate

import (
	"context"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/packet"
	"github.com/symsimmy/due/session"
	"github.com/symsimmy/due/transport"
	"github.com/symsimmy/due/transport/grpc/code"
	"github.com/symsimmy/due/transport/grpc/internal/pb"
	"github.com/symsimmy/due/transport/grpc/internal/server"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"time"
)

const defaultWriteChannelSize = 10240

func NewServer(provider transport.GateProvider, opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, err
	}

	endpoint := &endpoint{provider: provider}
	endpoint.multicastCh = make(chan *pb.MulticastRequest, defaultWriteChannelSize)
	endpoint.pushCh = make(chan *pb.PushRequest, defaultWriteChannelSize)
	endpoint.broadcastCh = make(chan *pb.BroadcastRequest, defaultWriteChannelSize)

	s.RegisterService(&pb.Gate_ServiceDesc, endpoint)

	go endpoint.dispatch()

	return s, nil
}

type endpoint struct {
	pb.UnimplementedGateServer
	provider    transport.GateProvider
	multicastCh chan *pb.MulticastRequest
	pushCh      chan *pb.PushRequest
	broadcastCh chan *pb.BroadcastRequest
}

// Bind 将用户与当前网关进行绑定
func (e *endpoint) Bind(ctx context.Context, req *pb.BindRequest) (*pb.BindReply, error) {
	err := e.provider.Bind(ctx, req.CID, req.UID)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	return &pb.BindReply{}, nil
}

// Unbind 将用户与当前网关进行解绑
func (e *endpoint) Unbind(ctx context.Context, req *pb.UnbindRequest) (*pb.UnbindReply, error) {
	err := e.provider.Unbind(ctx, req.UID)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	return &pb.UnbindReply{}, nil
}

// GetIP 获取客户端IP地址
func (e *endpoint) GetIP(ctx context.Context, req *pb.GetIPRequest) (*pb.GetIPReply, error) {
	ip, err := e.provider.GetIP(ctx, session.Kind(req.Kind), req.Target)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	return &pb.GetIPReply{IP: ip}, nil
}

// Stat 统计会话总数
func (e *endpoint) Stat(ctx context.Context, req *pb.StatRequest) (*pb.StatReply, error) {
	total, err := e.provider.Stat(ctx, session.Kind(req.Kind))
	if err != nil {
		return nil, err
	}

	return &pb.StatReply{Total: total}, nil
}

// IsOnline 用户是否在线
func (e *endpoint) IsOnline(ctx context.Context, req *pb.IsOnlineRequest) (*pb.IsOnlineReply, error) {
	isOnline, err := e.provider.IsOnline(ctx, session.Kind(req.Kind), req.Target)
	if err != nil {
		return nil, err
	}

	return &pb.IsOnlineReply{IsOnline: isOnline}, nil
}

// GetID 获取ID
func (e *endpoint) GetID(ctx context.Context, req *pb.GetIdRequest) (*pb.GetIdReply, error) {
	id, err := e.provider.GetID(ctx, session.Kind(req.Kind), req.Target)
	if err != nil {
		return nil, err
	}

	return &pb.GetIdReply{Id: id}, nil
}

// Push 推送消息给连接
func (e *endpoint) Push(stream pb.Gate_PushServer) error {
	for {
		//从流中获取消息
		req, err := stream.Recv()
		if err == io.EOF {
			//发送结果，并关闭
			return stream.SendAndClose(&pb.PushReply{})
		}
		if err != nil {
			log.Errorf("gate server receive push message failed.err:%v", err)
			return err
		}

		e.pushCh <- req
	}
}

// BatchPush 批量推送消息给连接
func (e *endpoint) BatchPush(ctx context.Context, req *pb.BatchPushRequest) (*pb.BatchPushReply, error) {
	for _, request := range req.Request {
		err := e.provider.Push(ctx, session.Kind(request.Kind), request.Target, &packet.Message{
			Seq:    request.Message.Seq,
			Route:  request.Message.Route,
			Buffer: request.Message.Buffer,
		})
		if err != nil {
			log.Warnf("method:[BatchPush] do request:[%+v] encountered error:[%v]", request, err.Error())
		}
	}

	return &pb.BatchPushReply{}, nil
}

// Multicast 推送组播消息
func (e *endpoint) Multicast(stream pb.Gate_MulticastServer) error {
	for {
		// 从流中获取消息
		req, err := stream.Recv()
		if err == io.EOF {
			// 发送结果，并关闭
			return stream.SendAndClose(&pb.MulticastReply{Total: 0})
		}
		if err != nil {
			log.Errorf("gate server receive multicast message failed.err:%v", err)
			return err
		}

		e.multicastCh <- req
	}
}

// BatchMulticast 批量推送组播消息
func (e *endpoint) BatchMulticast(ctx context.Context, req *pb.BatchMulticastRequest) (*pb.BatchMulticastReply, error) {
	for _, request := range req.Request {
		_, err := e.provider.Multicast(ctx, session.Kind(request.Kind), request.Targets, &packet.Message{
			Seq:    request.Message.Seq,
			Route:  request.Message.Route,
			Buffer: request.Message.Buffer,
		})
		if err != nil {
			log.Warnf("method:[BatchMulticast] do request:[%+v] encountered error:[%v]", request, err.Error())
		}
	}

	return &pb.BatchMulticastReply{}, nil
}

// Broadcast 推送广播消息
func (e *endpoint) Broadcast(stream pb.Gate_BroadcastServer) error {
	for {
		// 从流中获取消息
		req, err := stream.Recv()
		if err == io.EOF {
			// 发送结果，并关闭
			return stream.SendAndClose(&pb.BroadcastReply{Total: 0})
		}
		if err != nil {
			log.Errorf("gate server receive broadcast message failed.err:%v", err)
			return err
		}

		e.broadcastCh <- req
	}
}

// BatchBroadcast 批量推送广播消息
func (e *endpoint) BatchBroadcast(ctx context.Context, req *pb.BatchBroadcastRequest) (*pb.BatchBroadcastReply, error) {
	for _, request := range req.Request {
		_, err := e.provider.Broadcast(ctx, session.Kind(request.Kind), &packet.Message{
			Seq:    request.Message.Seq,
			Route:  request.Message.Route,
			Buffer: request.Message.Buffer,
		})
		if err != nil {
			log.Warnf("method:[BatchBroadcast] do request:[%+v] encountered error:[%v]", request, err.Error())
		}
	}

	return &pb.BatchBroadcastReply{}, nil
}

// Disconnect 断开连接
func (e *endpoint) Disconnect(ctx context.Context, req *pb.DisconnectRequest) (*pb.DisconnectReply, error) {
	err := e.provider.Disconnect(ctx, session.Kind(req.Kind), req.Target, req.IsForce)
	if err != nil {
		switch err {
		case session.ErrNotFoundSession:
			return nil, status.New(code.NotFoundSession, err.Error()).Err()
		case session.ErrInvalidSessionKind:
			return nil, status.New(codes.InvalidArgument, err.Error()).Err()
		default:
			return nil, status.New(codes.Internal, err.Error()).Err()
		}
	}

	return &pb.DisconnectReply{}, nil
}

func (e *endpoint) dispatch() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("dispatch multi panic: %v", r)
		}
	}()

	writeChBacklogTicker := time.NewTicker(1 * time.Second)
	defer writeChBacklogTicker.Stop()

	for {
		select {
		case <-writeChBacklogTicker.C:
			if len(e.multicastCh) >= defaultWriteChannelSize {
				log.Warnf("multicastCh size:%+v", len(e.multicastCh))
			}

			if len(e.pushCh) >= defaultWriteChannelSize {
				log.Warnf("pushCh size:%+v", len(e.pushCh))
			}

			if len(e.broadcastCh) >= defaultWriteChannelSize {
				log.Warnf("broadcastCh size:%+v", len(e.broadcastCh))
			}

		case req, ok := <-e.multicastCh:
			if !ok {
				return
			}

			go func(req *pb.MulticastRequest) {
				_, err := e.provider.Multicast(context.Background(), session.Kind(req.Kind), req.Targets, &packet.Message{
					Seq:    req.Message.Seq,
					Route:  req.Message.Route,
					Buffer: req.Message.Buffer,
				})

				if err != nil {
					log.Warnf("dispatch multicast message failed.err:%+v", err)
				}
			}(req)
		case req, ok := <-e.pushCh:
			if !ok {
				return
			}

			err := e.provider.Push(context.Background(), session.Kind(req.Kind), req.Target, &packet.Message{
				Seq:    req.Message.Seq,
				Route:  req.Message.Route,
				Buffer: req.Message.Buffer,
			})
			if err != nil {
				log.Warnf("dispatch push message failed.err:%+v", err)
			}

			if len(e.pushCh) >= defaultWriteChannelSize {
				log.Warnf("pushCh size:%+v", len(e.pushCh))
			}
		case req, ok := <-e.broadcastCh:
			if !ok {
				return
			}

			go func(req *pb.BroadcastRequest) {
				_, err := e.provider.Broadcast(context.Background(), session.Kind(req.Kind), &packet.Message{
					Seq:    req.Message.Seq,
					Route:  req.Message.Route,
					Buffer: req.Message.Buffer,
				})
				if err != nil {
					log.Warnf("dispatch broadcast message failed.err:%+v", err)
				}
			}(req)
		}
	}
}
