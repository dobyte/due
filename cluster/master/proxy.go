package master

import (
	"context"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/internal/link"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/session"
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

type Proxy interface {
	// GetID 获取当前节点ID
	GetID() string
	// GetName 获取当前节点名称
	GetName() string
	// LocateGate 定位用户所在网关
	LocateGate(ctx context.Context, uid int64) (string, error)
	// LocateNode 定位用户所在节点
	LocateNode(ctx context.Context, uid int64) (string, error)
	// FetchGateList 拉取网关列表
	FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error)
	// FetchNodeList 拉取节点列表
	FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error)
	// GetIP 获取客户端IP
	GetIP(ctx context.Context, uid int64) (string, error)
	// Push 推送消息
	Push(ctx context.Context, uid int64, message *Message) error
	// Multicast 推送组播消息
	Multicast(ctx context.Context, uids []int64, message *Message) (int64, error)
	// Broadcast 推送广播消息
	Broadcast(ctx context.Context, kind session.Kind, message *Message) (int64, error)
	// Deliver 投递消息给节点处理
	Deliver(ctx context.Context, uid int64, message *Message) error
	// Disconnect 断开连接
	Disconnect(ctx context.Context, uid int64, isForce bool) error
}

type proxy struct {
	master *Master
	link   *link.Link
}

func newProxy(master *Master) *proxy {
	return &proxy{master: master, link: link.NewLink(&link.Options{
		Codec:       master.opts.codec,
		Locator:     master.opts.locator,
		Registry:    master.opts.registry,
		Encryptor:   master.opts.encryptor,
		Transporter: master.opts.transporter,
	})}
}

// GetID 获取当前节点ID
func (p *proxy) GetID() string {
	return p.master.opts.id
}

// GetName 获取当前节点名称
func (p *proxy) GetName() string {
	return p.master.opts.name
}

// LocateGate 定位用户所在网关
func (p *proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	return p.link.LocateGate(ctx, uid)
}

// LocateNode 定位用户所在节点
func (p *proxy) LocateNode(ctx context.Context, uid int64) (string, error) {
	return p.link.LocateNode(ctx, uid)
}

// FetchGateList 拉取网关列表
func (p *proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.link.FetchServiceList(ctx, cluster.Gate, states...)
}

// FetchNodeList 拉取节点列表
func (p *proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.link.FetchServiceList(ctx, cluster.Node, states...)
}

// GetIP 获取客户端IP
func (p *proxy) GetIP(ctx context.Context, uid int64) (string, error) {
	return p.link.GetIP(ctx, &link.GetIPArgs{
		Kind:   session.User,
		Target: uid,
	})
}

// Push 推送消息
func (p *proxy) Push(ctx context.Context, uid int64, message *Message) error {
	return p.link.Push(ctx, &link.PushArgs{
		Kind:    session.User,
		Target:  uid,
		Message: message,
	})
}

// Multicast 推送组播消息
func (p *proxy) Multicast(ctx context.Context, uids []int64, message *Message) (int64, error) {
	return p.link.Multicast(ctx, &link.MulticastArgs{
		Kind:    session.User,
		Targets: uids[:],
		Message: message,
	})
}

// Broadcast 推送广播消息
func (p *proxy) Broadcast(ctx context.Context, kind session.Kind, message *Message) (int64, error) {
	return p.link.Broadcast(ctx, &link.BroadcastArgs{
		Kind:    kind,
		Message: message,
	})
}

// Deliver 投递消息给节点处理
func (p *proxy) Deliver(ctx context.Context, uid int64, message *Message) error {
	return p.link.Deliver(ctx, &link.DeliverArgs{
		UID:     uid,
		Message: message,
	})
}

// Disconnect 断开连接
func (p *proxy) Disconnect(ctx context.Context, uid int64, isForce bool) error {
	return p.link.Disconnect(ctx, &link.DisconnectArgs{
		Kind:    session.User,
		Target:  uid,
		IsForce: isForce,
	})
}

// 启动监听
func (p *proxy) watch(ctx context.Context) {
	p.link.WatchUserLocate(ctx, cluster.Gate, cluster.Node)

	p.link.WatchServiceInstance(ctx, cluster.Gate, cluster.Node)
}
