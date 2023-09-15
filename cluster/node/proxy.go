package node

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

type Proxy struct {
	node *Node      // 节点
	link *link.Link // 链接
}

func newProxy(node *Node) *Proxy {
	return &Proxy{node: node, link: link.NewLink(&link.Options{
		NID:         node.opts.id,
		Codec:       node.opts.codec,
		Locator:     node.opts.locator,
		Registry:    node.opts.registry,
		Encryptor:   node.opts.encryptor,
		Transporter: node.opts.transporter,
	})}
}

// GetNodeID 获取当前节点ID
func (p *Proxy) GetNodeID() string {
	return p.node.opts.id
}

// GetNodeName 获取当前节点名称
func (p *Proxy) GetNodeName() string {
	return p.node.opts.name
}

// GetNodeState 获取当前节点状态
func (p *Proxy) GetNodeState() cluster.State {
	return p.node.getState()
}

// SetNodeState 设置当前节点状态
func (p *Proxy) SetNodeState(state cluster.State) {
	p.node.setState(state)
}

// Router 路由器
func (p *Proxy) Router() *Router {
	return p.node.router
}

// Events 事件分发器
func (p *Proxy) Events() *Events {
	return p.node.events
}

// NewServiceClient 新建微服务客户端
// target参数可分为两种模式:
// 直连模式: 	direct://127.0.0.1:8011
// 服务发现模式: 	discovery://service_name
func (p *Proxy) NewServiceClient(target string) (transport.ServiceClient, error) {
	return p.node.opts.transporter.NewServiceClient(target)
}

// BindGate 绑定网关
func (p *Proxy) BindGate(ctx context.Context, uid int64, gid string, cid int64) error {
	return p.link.BindGate(ctx, uid, gid, cid)
}

// UnbindGate 解绑网关
func (p *Proxy) UnbindGate(ctx context.Context, uid int64) error {
	return p.link.UnbindGate(ctx, uid)
}

// BindNode 绑定节点
// 单个用户只能被绑定到某一台节点服务器上，多次绑定会直接覆盖上次绑定
// 绑定操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上
// NID 为需要绑定的节点ID，默认绑定到当前节点上
func (p *Proxy) BindNode(ctx context.Context, uid int64, nid ...string) error {
	if len(nid) == 0 || nid[0] == "" {
		return p.link.BindNode(ctx, uid, p.node.opts.id)
	} else {
		return p.link.BindNode(ctx, uid, nid[0])
	}
}

// UnbindNode 解绑节点
// 解绑时会对解绑节点ID进行校验，不匹配则解绑失败
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上
// NID 为需要解绑的节点ID，默认解绑当前节点
func (p *Proxy) UnbindNode(ctx context.Context, uid int64, nid ...string) error {
	if len(nid) == 0 || nid[0] == "" {
		return p.link.UnbindNode(ctx, uid, p.node.opts.id)
	} else {
		return p.link.UnbindNode(ctx, uid, nid[0])
	}
}

// LocateGate 定位用户所在网关
func (p *Proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	return p.link.LocateGate(ctx, uid)
}

// AskGate 检测用户是否在给定的网关上
func (p *Proxy) AskGate(ctx context.Context, uid int64, gid string) (string, bool, error) {
	return p.link.AskGate(ctx, uid, gid)
}

// LocateNode 定位用户所在节点
func (p *Proxy) LocateNode(ctx context.Context, uid int64) (string, error) {
	return p.link.LocateNode(ctx, uid)
}

// AskNode 检测用户是否在给定的节点上
func (p *Proxy) AskNode(ctx context.Context, uid int64, nid string) (string, bool, error) {
	return p.link.AskNode(ctx, uid, nid)
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
func (p *Proxy) GetIP(ctx context.Context, args *GetIPArgs) (string, error) {
	return p.link.GetIP(ctx, args)
}

// Push 推送消息
func (p *Proxy) Push(ctx context.Context, args *PushArgs) error {
	return p.link.Push(ctx, args)
}

// Multicast 推送组播消息
func (p *Proxy) Multicast(ctx context.Context, args *MulticastArgs) (int64, error) {
	return p.link.Multicast(ctx, args)
}

// Broadcast 推送广播消息
func (p *Proxy) Broadcast(ctx context.Context, args *BroadcastArgs) (int64, error) {
	return p.link.Broadcast(ctx, args)
}

// Deliver 投递消息给节点处理
func (p *Proxy) Deliver(ctx context.Context, args *DeliverArgs) error {
	if args.NID != p.GetNodeID() {
		return p.link.Deliver(ctx, &link.DeliverArgs{
			NID: args.NID,
			UID: args.UID,
			Message: &Message{
				Seq:   args.Message.Seq,
				Route: args.Message.Route,
				Data:  args.Message.Data,
			},
		})
	} else {
		p.node.router.deliver("", args.NID, 0, args.UID, args.Message.Seq, args.Message.Route, args.Message.Data)
	}

	return nil
}

// Response 响应消息
func (p *Proxy) Response(ctx context.Context, req *Request, message interface{}) error {
	switch {
	case req.GID != "":
		return p.link.Push(ctx, &link.PushArgs{
			GID:    req.GID,
			Kind:   session.Conn,
			Target: req.CID,
			Message: &Message{
				Seq:   req.Message.Seq,
				Route: req.Message.Route,
				Data:  message,
			},
		})
	case req.NID != "":
		return p.link.Deliver(ctx, &link.DeliverArgs{
			NID: req.NID,
			UID: req.UID,
			Message: &Message{
				Seq:   req.Message.Seq,
				Route: req.Message.Route,
				Data:  message,
			},
		})
	}

	return nil
}

// Disconnect 断开连接
func (p *Proxy) Disconnect(ctx context.Context, args *DisconnectArgs) error {
	return p.link.Disconnect(ctx, args)
}

// Invoke 调用函数（线程安全）
func (p *Proxy) Invoke(fn func()) {
	p.node.fnChan <- fn
}

// 启动监听
func (p *Proxy) watch(ctx context.Context) {
	p.link.WatchUserLocate(ctx, cluster.Gate, cluster.Node)

	p.link.WatchServiceInstance(ctx, cluster.Gate, cluster.Node)
}
