package master

import (
	"context"
	"github.com/symsimmy/due/cluster"
	"github.com/symsimmy/due/common/link"
	"github.com/symsimmy/due/registry"
	"github.com/symsimmy/due/session"
)

var (
	ErrInvalidGID         = link.ErrInvalidGID
	ErrInvalidNID         = link.ErrInvalidNID
	ErrInvalidMessage     = link.ErrInvalidMessage
	ErrInvalidArgument    = link.ErrInvalidArgument
	ErrInvalidSessionKind = link.ErrInvalidSessionKind
	ErrNotFoundUserSource = link.ErrNotFoundUserSource
	ErrReceiveTargetEmpty = link.ErrReceiveTargetEmpty
)

type (
	Message = link.Message
)

type Proxy struct {
	master *Master
	link   *link.Link
}

func newProxy(master *Master) *Proxy {
	return &Proxy{master: master, link: link.NewLink(&link.Options{
		Codec:       master.opts.codec,
		Locator:     master.opts.locator,
		Registry:    master.opts.registry,
		Encryptor:   master.opts.encryptor,
		Transporter: master.opts.transporter,
	})}
}

// GetMasterID 获取当前管理节点ID
func (p *Proxy) GetMasterID() string {
	return p.master.opts.id
}

// GetMasterName 获取当前管理节点名称
func (p *Proxy) GetMasterName() string {
	return p.master.opts.name
}

// LocateGate 定位用户所在网关
func (p *Proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	return p.link.LocateGate(ctx, uid)
}

// LocateNode 定位用户所在节点
func (p *Proxy) LocateNode(ctx context.Context, uid int64) (string, error) {
	return p.link.LocateNode(ctx, uid)
}

// FetchGateList 拉取网关列表
func (p *Proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.link.FetchServiceList(ctx, cluster.Gate, states...)
}

// FetchNodeList 拉取节点列表
func (p *Proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.link.FetchServiceList(ctx, cluster.Node, states...)
}

// FetchGameList 拉取游戏节点列表
func (p *Proxy) FetchGameList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.link.FetchServiceAliasList(ctx, cluster.Node, "game", states...)
}

// FetchCenterList 拉取Center节点列表
func (p *Proxy) FetchCenterList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.link.FetchServiceAliasList(ctx, cluster.Node, "center", states...)
}

// GetIP 获取客户端IP
func (p *Proxy) GetIP(ctx context.Context, uid int64) (string, error) {
	return p.link.GetIP(ctx, &link.GetIPArgs{
		Kind:   session.User,
		Target: uid,
	})
}

// Push 推送消息
func (p *Proxy) Push(ctx context.Context, uid int64, message *Message) error {
	return p.link.Push(ctx, &link.PushArgs{
		Kind:    session.User,
		Target:  uid,
		Message: message,
	})
}

// Multicast 推送组播消息
func (p *Proxy) Multicast(ctx context.Context, uids []int64, message *Message) (int64, error) {
	return p.link.Multicast(ctx, &link.MulticastArgs{
		Kind:    session.User,
		Targets: uids[:],
		Message: message,
	})
}

// Broadcast 推送广播消息
func (p *Proxy) Broadcast(ctx context.Context, kind session.Kind, message *Message) (int64, error) {
	return p.link.Broadcast(ctx, &link.BroadcastArgs{
		Kind:    kind,
		Message: message,
	})
}

// BroadcastDeliver 推送广播消息到node节点
func (p *Proxy) BroadcastDeliver(ctx context.Context, kind link.DeliverKind, message *Message) error {
	return p.link.BroadcastDeliver(ctx, &link.BroadcastDeliverArgs{
		Kind:    kind,
		Message: message,
	})
}

// MulticastDeliver 推送广播消息到node节点
func (p *Proxy) MulticastDeliver(ctx context.Context, kind link.DeliverKind, targets []string, message *Message) error {
	return p.link.MulticastDeliver(ctx, &link.MulticastDeliverArgs{
		Kind:    kind,
		Targets: targets,
		Message: message,
	})
}

// Deliver 投递消息给节点处理
func (p *Proxy) Deliver(ctx context.Context, uid int64, message *Message) error {
	return p.link.Deliver(ctx, &link.DeliverArgs{
		UID:     uid,
		Message: message,
	})
}

// DeliverN 通过nodeId投递消息给节点处理
func (p *Proxy) DeliverN(ctx context.Context, nid string, message *Message) error {
	return p.link.Deliver(ctx, &link.DeliverArgs{
		NID:     nid,
		Message: message,
	})
}

// BlockConn
func (p *Proxy) BlockConn(ctx context.Context, onid string, nnid string, target uint64) {
	p.link.BlockConn(ctx, onid, nnid, target)
}

// Disconnect 断开连接
func (p *Proxy) Disconnect(ctx context.Context, uid int64, isForce bool) error {
	return p.link.Disconnect(ctx, &link.DisconnectArgs{
		Kind:    session.User,
		Target:  uid,
		IsForce: isForce,
	})
}

// 启动监听
func (p *Proxy) watch(ctx context.Context) {
	p.link.WatchUserLocate(ctx, cluster.Gate, cluster.Node)

	p.link.WatchServiceInstance(ctx, cluster.Gate, cluster.Node)
}
