package node

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
	GetIPArgs      = link.GetIPArgs
	PushArgs       = link.PushArgs
	MulticastArgs  = link.MulticastArgs
	BroadcastArgs  = link.BroadcastArgs
	DisconnectArgs = link.DisconnectArgs
	Message        = link.Message
)

type DeliverArgs struct {
	NID     string   // 接收节点。存在接收节点时，消息会直接投递给接收节点；不存在接收节点时，系统定位用户所在节点，然后投递。
	UID     int64    // 用户ID
	Message *Message // 消息
}

type Proxy interface {
	// GetID 获取当前节点ID
	GetID() string
	// AddRouteHandler 添加路由处理器
	AddRouteHandler(route int32, stateful bool, handler RouteHandler)
	// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
	SetDefaultRouteHandler(handler RouteHandler)
	// AddEventListener 添加事件监听器
	AddEventListener(event cluster.Event, handler EventHandler)
	// BindGate 绑定网关
	BindGate(ctx context.Context, gid string, cid, uid int64) error
	// UnbindGate 绑定网关
	UnbindGate(ctx context.Context, uid int64) error
	// BindNode 绑定节点
	BindNode(ctx context.Context, uid int64, nid ...string) error
	// UnbindNode 解绑节点
	UnbindNode(ctx context.Context, uid int64, nid ...string) error
	// LocateGate 定位用户所在网关
	LocateGate(ctx context.Context, uid int64) (string, error)
	// LocateNode 定位用户所在节点
	LocateNode(ctx context.Context, uid int64) (string, error)
	// FetchGateList 拉取网关列表
	FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error)
	// FetchNodeList 拉取节点列表
	FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error)
	// GetIP 获取客户端IP
	GetIP(ctx context.Context, args *GetIPArgs) (string, error)
	// Push 推送消息
	Push(ctx context.Context, args *PushArgs) error
	// Multicast 推送组播消息
	Multicast(ctx context.Context, args *MulticastArgs) (int64, error)
	// Broadcast 推送广播消息
	Broadcast(ctx context.Context, args *BroadcastArgs) (int64, error)
	// Deliver 投递消息给节点处理
	Deliver(ctx context.Context, args *DeliverArgs) error
	// Response 响应消息
	Response(ctx context.Context, req Request, message interface{}) error
	// Disconnect 断开连接
	Disconnect(ctx context.Context, args *DisconnectArgs) error
}

type proxy struct {
	node *Node // 节点
	link *link.Link
}

func newProxy(node *Node) *proxy {
	return &proxy{
		node: node,
		link: link.NewLink(&link.Options{
			NID:         node.opts.id,
			Codec:       node.opts.codec,
			Locator:     node.opts.locator,
			Registry:    node.opts.registry,
			Encryptor:   node.opts.encryptor,
			Transporter: node.opts.transporter,
		}),
	}
}

// GetID 获取当前节点ID
func (p *proxy) GetID() string {
	return p.node.opts.id
}

// AddRouteHandler 添加路由处理器
func (p *proxy) AddRouteHandler(route int32, stateful bool, handler RouteHandler) {
	p.node.addRouteHandler(route, stateful, handler)
}

// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
func (p *proxy) SetDefaultRouteHandler(handler RouteHandler) {
	p.node.setDefaultRouteHandler(handler)
}

// AddEventListener 添加事件监听器
func (p *proxy) AddEventListener(event cluster.Event, handler EventHandler) {
	p.node.addEventListener(event, handler)
}

// BindGate 绑定网关
func (p *proxy) BindGate(ctx context.Context, gid string, cid, uid int64) error {
	return p.link.BindGate(ctx, gid, cid, uid)
}

// UnbindGate 解绑网关
func (p *proxy) UnbindGate(ctx context.Context, uid int64) error {
	return p.link.UnbindGate(ctx, uid)
}

// BindNode 绑定节点
// 单个用户只能被绑定到某一台节点服务器上，多次绑定会直接覆盖上次绑定
// 绑定操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上
// nid 为需要绑定的节点ID，默认绑定到当前节点上
func (p *proxy) BindNode(ctx context.Context, uid int64, nid ...string) error {
	if len(nid) == 0 || nid[0] == "" {
		return p.link.BindNode(ctx, uid, p.node.opts.id)
	} else {
		return p.link.BindNode(ctx, uid, nid[0])
	}
}

// UnbindNode 解绑节点
// 解绑时会对解绑节点ID进行校验，不匹配则解绑失败
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上
// nid 为需要解绑的节点ID，默认解绑当前节点
func (p *proxy) UnbindNode(ctx context.Context, uid int64, nid ...string) error {
	if len(nid) == 0 || nid[0] == "" {
		return p.link.UnbindNode(ctx, uid, p.node.opts.id)
	} else {
		return p.link.UnbindNode(ctx, uid, nid[0])
	}
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
func (p *proxy) GetIP(ctx context.Context, args *GetIPArgs) (string, error) {
	return p.link.GetIP(ctx, args)
}

// Push 推送消息
func (p *proxy) Push(ctx context.Context, args *PushArgs) error {
	return p.link.Push(ctx, args)
}

// Multicast 推送组播消息
func (p *proxy) Multicast(ctx context.Context, args *MulticastArgs) (int64, error) {
	return p.link.Multicast(ctx, args)
}

// Broadcast 推送广播消息
func (p *proxy) Broadcast(ctx context.Context, args *BroadcastArgs) (int64, error) {
	return p.link.Broadcast(ctx, args)
}

// Deliver 投递消息给节点处理
func (p *proxy) Deliver(ctx context.Context, args *DeliverArgs) error {
	message := &Message{
		Seq:   args.Message.Seq,
		Route: args.Message.Route,
		Data:  args.Message.Data,
	}

	if args.NID != p.GetID() {
		return p.link.Deliver(ctx, &link.DeliverArgs{
			NID:     args.NID,
			UID:     args.UID,
			Message: message,
		})
	}

	p.node.deliver(&request{
		nid:     args.NID,
		uid:     args.UID,
		message: message,
	})

	return nil
}

// Response 响应消息
func (p *proxy) Response(ctx context.Context, req Request, message interface{}) error {
	switch {
	case req.GID() != "":
		return p.link.Push(ctx, &link.PushArgs{
			GID:    req.GID(),
			Kind:   session.Conn,
			Target: req.CID(),
			Message: &Message{
				Seq:   req.Seq(),
				Route: req.Route(),
				Data:  message,
			},
		})
	case req.NID() != "":
		return p.link.Deliver(ctx, &link.DeliverArgs{
			NID: req.NID(),
			UID: req.UID(),
			Message: &Message{
				Seq:   req.Seq(),
				Route: req.Route(),
				Data:  message,
			},
		})
	}

	return nil
}

// Disconnect 断开连接
func (p *proxy) Disconnect(ctx context.Context, args *DisconnectArgs) error {
	return p.link.Disconnect(ctx, args)
}

// 启动监听
func (p *proxy) watch(ctx context.Context) {
	p.link.WatchUserLocate(ctx, cluster.Gate, cluster.Node)

	p.link.WatchServiceInstance(ctx, string(cluster.Gate))

	p.link.WatchServiceInstance(ctx, string(cluster.Node))
}
