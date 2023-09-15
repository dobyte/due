package master

import (
	"context"
	"github.com/symsimmy/due/cluster"
	"github.com/symsimmy/due/internal/link"
	"github.com/symsimmy/due/registry"
	"github.com/symsimmy/due/session"
	"github.com/symsimmy/due/transport"
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

// NewServiceClient 新建微服务客户端
// target参数可分为两种模式:
// 直连模式: 	direct://127.0.0.1:8011
// 服务发现模式: 	discovery://service_name
func (p *Proxy) NewServiceClient(target string) (transport.ServiceClient, error) {
	return p.master.opts.transporter.NewServiceClient(target)
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

// Deliver 投递消息给节点处理
func (p *Proxy) Deliver(ctx context.Context, uid int64, message *Message) error {
	return p.link.Deliver(ctx, &link.DeliverArgs{
		UID:     uid,
		Message: message,
	})
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
