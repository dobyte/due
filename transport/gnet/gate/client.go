package gate

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/session"
	"github.com/symsimmy/due/transport"
	"github.com/symsimmy/due/transport/gnet/internal/pb"
	"github.com/symsimmy/due/transport/gnet/tcp"
	"time"
)

type Client struct {
	client *tcp.Client
}

func NewClient(cc *tcp.Client) *Client {
	return &Client{client: cc}
}

func (c *Client) Ping(ctx context.Context, message string) (replyMessage string, err error) {
	req := &pb.PingRequest{
		Message: message,
	}
	reply := &pb.PingReply{}
	dataCh, err := c.client.SendWithReply(transport.Ping, req)
	if err != nil {
		return
	}

	err = WaitChData(dataCh, reply)
	if err != nil {
		return
	}

	if reply.ErrorCode != 0 || len(reply.ErrorMessage) > 0 {
		log.Warnf("Ping request receive reply with err.reply:%+v.", reply)
		return "", errors.New(reply.ErrorMessage)
	}

	replyMessage = reply.Reply
	return
}

func (c *Client) GetIP(ctx context.Context, kind session.Kind, target int64) (ip string, miss bool, err error) {
	req := &pb.GetIPRequest{
		Kind:   int32(kind),
		Target: target,
	}
	reply := &pb.GetIPReply{}
	dataCh, err := c.client.SendWithReply(transport.GetIP, req)
	if err != nil {
		return
	}

	err = WaitChData(dataCh, reply)
	if err != nil {
		return
	}

	if reply.ErrorCode != 0 || len(reply.ErrorMessage) > 0 {
		log.Warnf("Kind:%+v,Target:%+v,GetIP request receive reply with err.reply:%+v.", kind, target, reply)
		return "", false, errors.New(reply.ErrorMessage)
	}

	ip = reply.IP
	miss = false
	return
}

func (c *Client) Disconnect(ctx context.Context, kind session.Kind, target int64, isForce bool) (miss bool, err error) {
	req := &pb.DisconnectRequest{
		Kind:    int32(kind),
		Target:  target,
		IsForce: isForce,
	}
	reply := &pb.DisconnectReply{}
	dataCh, err := c.client.SendWithReply(transport.Disconnect, req)
	if err != nil {
		return false, err
	}

	err = WaitChData(dataCh, reply)
	if err != nil {
		return false, err
	}

	if reply.ErrorCode != 0 || len(reply.ErrorMessage) > 0 {
		log.Warnf("Kind:%+v,Target:%+v,IsForce:%+v,Disconnect request receive reply with err.reply:%+v.", kind, target, isForce, reply)
		return false, errors.New(reply.ErrorMessage)
	}

	miss = false
	return
}

func (c *Client) Stat(ctx context.Context, kind session.Kind) (total int64, miss bool, err error) {
	req := &pb.StatRequest{
		Kind: int32(kind),
	}
	reply := &pb.StatReply{}
	dataCh, err := c.client.SendWithReply(transport.Stat, req)
	if err != nil {
		return 0, false, err
	}

	err = WaitChData(dataCh, reply)
	if err != nil {
		return 0, false, err
	}

	if reply.ErrorCode != 0 || len(reply.ErrorMessage) > 0 {
		log.Warnf("Kind:%+v,Stat request receive reply with err.reply:%+v.", kind, reply)
		return 0, false, errors.New(reply.ErrorMessage)
	}

	total = reply.Total
	miss = false
	return
}

func (c *Client) IsOnline(ctx context.Context, kind session.Kind, target int64) (isOnline, miss bool, err error) {
	req := &pb.IsOnlineRequest{
		Kind:   int32(kind),
		Target: target,
	}
	reply := &pb.IsOnlineReply{}
	dataCh, err := c.client.SendWithReply(transport.IsOnline, req)
	if err != nil {
		return false, false, err
	}

	err = WaitChData(dataCh, reply)
	if err != nil {
		return false, false, err
	}

	if reply.ErrorCode != 0 || len(reply.ErrorMessage) > 0 {
		log.Warnf("Kind:%+v,Target:%+v,IsOnline request receive reply with err.reply:%+v.", kind, target, reply)
		return false, false, errors.New(reply.ErrorMessage)
	}

	isOnline = reply.IsOnline
	miss = false
	return
}

func (c *Client) GetID(ctx context.Context, kind session.Kind, target int64) (id int64, err error) {
	req := &pb.GetIdRequest{
		Kind:   int32(kind),
		Target: target,
	}
	reply := &pb.GetIdReply{}
	dataCh, err := c.client.SendWithReply(transport.GetID, req)

	if err != nil {
		return 0, err
	}

	err = WaitChData(dataCh, reply)
	if err != nil {
		return 0, err
	}

	if reply.ErrorCode != 0 || len(reply.ErrorMessage) > 0 {
		log.Warnf("Kind:%+v,Target:%+v,GetID request receive reply with err.reply:%+v.", kind, target, reply)
		return 0, errors.New(reply.ErrorMessage)
	}

	id = reply.Id
	return
}

// Trigger 触发事件
func (c *Client) Trigger(ctx context.Context, args *transport.TriggerArgs) (miss bool, err error) {
	req := &pb.TriggerRequest{
		Event: int32(args.Event),
		GID:   args.GID,
		CID:   args.CID,
		UID:   args.UID,
	}

	err = c.client.Send(transport.Trigger, req)
	miss = false

	return
}

// Bind 绑定用户与连接
func (c *Client) Bind(ctx context.Context, cid, uid int64) (miss bool, err error) {
	req := &pb.BindRequest{
		CID: cid,
		UID: uid,
	}
	reply := &pb.BindReply{}
	dataCh, err := c.client.SendWithReply(transport.Bind, req)
	if err != nil {
		return false, err
	}

	err = WaitChData(dataCh, reply)
	if reply.ErrorCode != 0 || len(reply.ErrorMessage) > 0 {
		log.Warnf("CID:%+v,UID:%+v,Bind request receive reply with err.reply:%+v.", cid, uid, reply)
		return false, errors.New(reply.ErrorMessage)
	}
	return false, err
}

// Unbind 解绑用户与连接
func (c *Client) Unbind(ctx context.Context, uid int64) (miss bool, err error) {
	req := &pb.UnbindRequest{
		UID: uid,
	}
	reply := &pb.BindReply{}
	dataCh, err := c.client.SendWithReply(transport.Unbind, req)
	if err != nil {
		return false, err
	}

	err = WaitChData(dataCh, reply)
	if reply.ErrorCode != 0 || len(reply.ErrorMessage) > 0 {
		log.Warnf("UID:%+v,Unbind request receive reply with err.reply:%+v.", uid, reply)
		return false, errors.New(reply.ErrorMessage)
	}
	return false, err
}

// Push 推送消息
func (c *Client) Push(ctx context.Context, kind session.Kind, target int64, message *transport.Message) (miss bool, err error) {
	req := &pb.PushRequest{
		Kind:   int32(kind),
		Target: target,
		Message: &pb.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		},
	}

	err = c.client.Send(transport.Push, req)
	miss = false

	return
}

// Multicast 推送组播消息
func (c *Client) Multicast(ctx context.Context, kind session.Kind, targets []int64, message *transport.Message) (total int64, err error) {
	req := &pb.MulticastRequest{
		Kind:    int32(kind),
		Targets: targets,
		Message: &pb.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		},
	}

	err = c.client.Send(transport.Multicast, req)
	return 0, err
}

// Broadcast 推送广播消息
func (c *Client) Broadcast(ctx context.Context, kind session.Kind, message *transport.Message) (total int64, err error) {
	req := &pb.BroadcastRequest{
		Kind: int32(kind),
		Message: &pb.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		},
	}

	err = c.client.Send(transport.Broadcast, req)
	return 0, err
}

func (c *Client) BlockConn(ctx context.Context, onid string, nnid string, target uint64) (err error) {
	req := &pb.BlockConnRequest{
		ONid:   onid,
		NNid:   nnid,
		Target: target,
	}

	err = c.client.Send(transport.BlockConn, req)
	return err
}

func WaitChData(packet *tcp.ReplyPacket, reply proto.Message) error {
	select {
	case data, ok := <-packet.ReplyCh:
		if !ok {
			return errors.New("receive channel is closed")
		}
		err := proto.Unmarshal(data, reply)
		log.Debugf("messageId:%+v,methodName:%+v,req:%+v,receive reply:%+v.", packet.MessageId, packet.MethodName, packet.Req, reply)
		return err
	case <-time.After(2 * time.Second):
		log.Warnf("messageId:%+v,methodName:%+v,req:%+v,receive reply timeout.", packet.MessageId, packet.MethodName, packet.Req)
		return errors.New("receive message timeout")
	}
}
