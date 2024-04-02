package gate

import (
	"context"
	"github.com/symsimmy/due/cluster"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/packet"
	"github.com/symsimmy/due/session"
	"strconv"
	"strings"
)

type provider struct {
	gate *Gate
}

// Bind 绑定用户与网关间的关系
func (p *provider) Bind(ctx context.Context, cid, uid int64) error {
	if cid <= 0 || uid <= 0 {
		return ErrInvalidArgument
	}

	err := p.gate.session.Bind(cid, uid)
	if err != nil {
		return err
	}

	err = p.gate.proxy.bindGate(ctx, cid, uid)
	if err != nil {
		_, _ = p.gate.session.Unbind(uid)
	}

	log.Infof("cid:%+v,uid:%+v bind to gate %+v.", cid, uid, p.gate.opts.id)

	return err
}

// Unbind 解绑用户与网关间的关系
func (p *provider) Unbind(ctx context.Context, uid int64) error {
	if uid == 0 {
		return ErrInvalidArgument
	}

	cid, err := p.gate.session.Unbind(uid)
	if err != nil {
		return err
	}

	err = p.gate.proxy.unbindGate(ctx, cid, uid)
	if err != nil {
		return err
	}

	log.Infof("cid:%+v,uid:%+v unbind to gate.", cid, uid)

	return nil
}

// GetIP 获取客户端IP地址
func (p *provider) GetIP(ctx context.Context, kind session.Kind, target int64) (string, error) {
	return p.gate.session.RemoteIP(kind, target)
}

// IsOnline 检测是否在线
func (p *provider) IsOnline(ctx context.Context, kind session.Kind, target int64) (bool, error) {
	return p.gate.session.Has(kind, target)
}

func (p *provider) GetID(ctx context.Context, kind session.Kind, target int64) (id int64, err error) {
	return p.gate.session.ID(kind, target)
}

// ID 获取conn的ID
func (p *provider) ID(ctx context.Context, kind session.Kind, target int64) (int64, error) {
	return p.gate.session.ID(kind, target)
}

// Push 发送消息
func (p *provider) Push(ctx context.Context, kind session.Kind, target int64, message *packet.Message) error {
	msg, err := packet.Pack(message)
	if err != nil {
		return err
	}

	conn, err := p.gate.session.Conn(kind, target)
	if err != nil {
		return err
	}
	log.Debugf("dispatch push message from server.Kind:%+v,Seq:%+v,Route:%+v,targets:%+v,len:%+v",
		kind,
		message.Seq,
		message.Route,
		conn.UID(),
		len(message.Buffer),
	)

	err = p.gate.session.Push(kind, target, msg)
	if kind == session.User && err == session.ErrNotFoundSession {
		err = p.gate.opts.locator.Rem(ctx, target, cluster.Gate, p.gate.opts.id)
		if err != nil {
			return err
		}
	}

	return err
}

// Multicast 推送组播消息
func (p *provider) Multicast(ctx context.Context, kind session.Kind, targets []int64, message *packet.Message) (int64, error) {
	if len(targets) == 0 {
		return 0, nil
	}

	//if message.Route != 6014 && message.Route != 6514 && message.Route != 6515 {
	stringSlice := make([]string, len(targets))

	for i, v := range targets {
		stringSlice[i] = strconv.FormatInt(v, 10)
	}

	targetsStr := strings.Join(stringSlice, ",")
	log.Debugf("dispatch multicast message from server.Kind:%+v,Seq:%+v,Route:%+v,targets:%+v,len:%+v",
		kind,
		message.Seq,
		message.Route,
		targetsStr,
		len(message.Buffer),
	)
	//}

	msg, err := packet.Pack(message)
	if err != nil {
		return 0, err
	}

	n, err := p.gate.session.Multicast(kind, targets, msg)
	return n, err
}

// Broadcast 推送广播消息
func (p *provider) Broadcast(ctx context.Context, kind session.Kind, message *packet.Message) (int64, error) {
	msg, err := packet.Pack(message)
	if err != nil {
		return 0, err
	}
	//if message.Route != 6014 && message.Route != 6514 && message.Route != 6515 {
	log.Debugf("dispatch broadcast message from server.Kind:%+v,Seq:%+v,Route:%+v,len:%+v",
		kind,
		message.Seq,
		message.Route,
		len(message.Buffer),
	)
	//}

	n, err := p.gate.session.Broadcast(kind, msg)
	return n, err
}

// Block
func (p *provider) Block(ctx context.Context, oNid string, nNid string, target uint64) {
	log.Infof("block target[%v] oNid[%v] nNid[%v]", target, oNid, nNid)
	conn, err := p.gate.session.Conn(session.User, int64(target))
	if err != nil {
		log.Warnf("block target[%v] oNid[%v] nNid[%v] err [%v]", target, oNid, nNid, err)
		return
	}
	conn.Block()
	p.gate.proxy.link.BindNode(ctx, int64(target), nNid)
	// 触发服务器数据迁移
	//p.gate.proxy.deliverN(ctx, oNid, &link.Message{Route: route.S2s_server_transport_request, Data: &pb.S2SServerTransportRequest{ONid: oNid, NNid: nNid, Uid: target}})
}

// Release
func (p *provider) Release(ctx context.Context, target uint64) {
	log.Infof("release target[%v]", target)
	conn, err := p.gate.session.Conn(session.User, int64(target))
	if err != nil {
		log.Warnf("release target[%v] err[%v]", target, err)
	}
	conn.Release()
}

// Stat 统计会话总数
func (p *provider) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	return p.gate.session.Stat(kind)
}

// Disconnect 断开连接
func (p *provider) Disconnect(ctx context.Context, kind session.Kind, target int64, isForce bool) error {
	return p.gate.session.Close(kind, target, isForce)
}
