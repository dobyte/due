package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport"
)

type Proxy struct {
	node       *Node            // 节点服务器
	gateLinker *link.GateLinker // 网关链接器
	nodeLinker *link.NodeLinker // 节点链接器
}

func newProxy(node *Node) *Proxy {
	opts := &link.Options{
		InsID:     node.opts.id,
		InsKind:   cluster.Node,
		Codec:     node.opts.codec,
		Locator:   node.opts.locator,
		Registry:  node.opts.registry,
		Encryptor: node.opts.encryptor,
	}

	return &Proxy{
		node:       node,
		gateLinker: link.NewGateLinker(node.opts.ctx, opts),
		nodeLinker: link.NewNodeLinker(node.opts.ctx, opts),
	}
}

// GetID 获取当前节点ID
func (p *Proxy) GetID() string {
	return p.node.opts.id
}

// GetName 获取当前节点名称
func (p *Proxy) GetName() string {
	return p.node.opts.name
}

// GetState 获取当前节点状态
func (p *Proxy) GetState() cluster.State {
	return p.node.getState()
}

// SetState 设置当前节点状态
func (p *Proxy) SetState(state cluster.State) error {
	return p.node.updateState(state)
}

// Router 路由器
func (p *Proxy) Router() *Router {
	return p.node.router
}

// RouteGroup 路由组
func (p *Proxy) RouteGroup(groups ...func(group *RouterGroup)) *RouterGroup {
	return p.node.router.Group(groups...)
}

// Trigger 事件触发器
func (p *Proxy) Trigger() *Trigger {
	return p.node.trigger
}

// AddRouteHandler 添加路由处理器
func (p *Proxy) AddRouteHandler(route int32, stateful bool, handler RouteHandler, middlewares ...MiddlewareHandler) {
	p.node.router.AddRouteHandler(route, stateful, handler, middlewares...)
}

// AddInternalRouteHandler 添加内部路由处理器（node节点间路由消息处理）
func (p *Proxy) AddInternalRouteHandler(route int32, stateful bool, handler RouteHandler, middlewares ...MiddlewareHandler) {
	p.node.router.AddInternalRouteHandler(route, stateful, handler, middlewares...)
}

// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
func (p *Proxy) SetDefaultRouteHandler(handler RouteHandler) {
	p.node.router.SetDefaultRouteHandler(handler)
}

// AddEventHandler 添加事件处理器
func (p *Proxy) AddEventHandler(event cluster.Event, handler EventHandler) {
	p.node.trigger.AddEventHandler(event, handler)
}

// AddHookListener 添加钩子监听器
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.node.addHookListener(hook, handler)
}

// AddServiceProvider 添加服务提供者
func (p *Proxy) AddServiceProvider(name string, desc interface{}, provider interface{}) {
	p.node.addServiceProvider(name, desc, provider)
}

// NewMeshClient 新建微服务客户端
// target参数可分为三种种模式:
// 服务直连模式: 	direct://127.0.0.1:8011
// 服务直连模式: 	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
// 服务发现模式: 	discovery://service_name
func (p *Proxy) NewMeshClient(target string) (transport.Client, error) {
	if p.node.opts.transporter == nil {
		return nil, errors.ErrMissTransporter
	}

	return p.node.opts.transporter.NewClient(target)
}

// BindGate 绑定网关
func (p *Proxy) BindGate(ctx context.Context, gid string, cid, uid int64) error {
	return p.gateLinker.Bind(ctx, gid, cid, uid)
}

// UnbindGate 解绑网关
func (p *Proxy) UnbindGate(ctx context.Context, uid int64) error {
	return p.gateLinker.Unbind(ctx, uid)
}

// BindNode 绑定节点
// 单个用户可以绑定到多个节点服务器上，相同名称的节点服务器只能绑定一个，多次绑定会到相同名称的节点服务器会覆盖之前的绑定。
// 绑定操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (p *Proxy) BindNode(ctx context.Context, uid int64, nameAndNID ...string) error {
	if len(nameAndNID) >= 2 && nameAndNID[0] != "" && nameAndNID[1] != "" {
		return p.nodeLinker.Bind(ctx, uid, nameAndNID[0], nameAndNID[1])
	} else {
		return p.nodeLinker.Bind(ctx, uid, p.node.opts.name, p.node.opts.id)
	}
}

// UnbindNode 解绑节点
// 解绑时会对对应名称的节点服务器进行解绑，解绑时会对解绑节点ID进行校验，不匹配则解绑失败。
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (p *Proxy) UnbindNode(ctx context.Context, uid int64, nameAndNID ...string) error {
	if len(nameAndNID) >= 2 && nameAndNID[0] != "" && nameAndNID[1] != "" {
		return p.nodeLinker.Unbind(ctx, uid, nameAndNID[0], nameAndNID[1])
	} else {
		return p.nodeLinker.Unbind(ctx, uid, p.node.opts.name, p.node.opts.id)
	}
}

// LocateGate 定位用户所在网关
func (p *Proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	return p.gateLinker.Locate(ctx, uid)
}

// AskGate 检测用户是否在给定的网关上
func (p *Proxy) AskGate(ctx context.Context, gid string, uid int64) (string, bool, error) {
	return p.gateLinker.Ask(ctx, gid, uid)
}

// HasGate 检测是否存在某个网关
func (p *Proxy) HasGate(gid string) bool {
	return p.gateLinker.Has(gid)
}

// LocateNode 定位用户所在节点
func (p *Proxy) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	return p.nodeLinker.Locate(ctx, uid, name)
}

// AskNode 检测用户是否在给定的节点上
func (p *Proxy) AskNode(ctx context.Context, uid int64, name, nid string) (string, bool, error) {
	return p.nodeLinker.Ask(ctx, uid, name, nid)
}

// HasNode 检测是否存在某个节点
func (p *Proxy) HasNode(nid string) bool {
	return p.nodeLinker.Has(nid)
}

// FetchGateList 拉取网关列表
func (p *Proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.gateLinker.FetchGateList(ctx, states...)
}

// FetchNodeList 拉取节点列表
func (p *Proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.nodeLinker.FetchNodeList(ctx, states...)
}

// BindActor 绑定Actor
func (p *Proxy) BindActor(uid int64, kind, id string) error {
	return p.node.scheduler.bindActor(uid, kind, id)
}

// UnbindActor 解绑Actor
func (p *Proxy) UnbindActor(uid int64, kind string) {
	p.node.scheduler.unbindActor(uid, kind)
}

// PackMessage 打包消息
func (p *Proxy) PackMessage(message *cluster.Message) ([]byte, error) {
	buf, err := p.gateLinker.PackMessage(message, true)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// PackBuffer 打包Buffer
func (p *Proxy) PackBuffer(message any) ([]byte, error) {
	return p.gateLinker.PackBuffer(message, true)
}

// GetIP 获取客户端IP
func (p *Proxy) GetIP(ctx context.Context, args *cluster.GetIPArgs) (string, error) {
	return p.gateLinker.GetIP(ctx, args)
}

// Stat 统计会话总数
func (p *Proxy) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	return p.gateLinker.Stat(ctx, kind)
}

// IsOnline 检测是否在线
func (p *Proxy) IsOnline(ctx context.Context, args *cluster.IsOnlineArgs) (bool, error) {
	return p.gateLinker.IsOnline(ctx, args)
}

// Disconnect 断开连接
func (p *Proxy) Disconnect(ctx context.Context, args *cluster.DisconnectArgs) error {
	return p.gateLinker.Disconnect(ctx, args)
}

// Push 推送消息
func (p *Proxy) Push(ctx context.Context, args *cluster.PushArgs) error {
	return p.gateLinker.Push(ctx, args)
}

// Multicast 推送组播消息
func (p *Proxy) Multicast(ctx context.Context, args *cluster.MulticastArgs) error {
	return p.gateLinker.Multicast(ctx, args)
}

// Broadcast 推送广播消息
func (p *Proxy) Broadcast(ctx context.Context, args *cluster.BroadcastArgs) error {
	return p.gateLinker.Broadcast(ctx, args)
}

// Deliver 投递消息给节点处理
func (p *Proxy) Deliver(ctx context.Context, args *cluster.DeliverArgs) error {
	if args.NID == p.node.opts.id {
		return errors.ErrIllegalOperation
	}

	return p.nodeLinker.Deliver(ctx, &link.DeliverArgs{
		NID:     args.NID,
		UID:     args.UID,
		Route:   args.Message.Route,
		Message: args.Message,
	})
}

// Invoke 调用函数（线程安全）
func (p *Proxy) Invoke(fn func()) {
	p.node.fnChan <- fn
}

// Spawn 衍生出一个新的Actor
func (p *Proxy) Spawn(creator Creator, opts ...ActorOption) (*Actor, error) {
	return p.node.scheduler.spawn(creator, opts...)
}

// Kill 杀死存在的一个Actor
func (p *Proxy) Kill(kind, id string) bool {
	return p.node.scheduler.kill(kind, id)
}

// Actor 获取Actor
func (p *Proxy) Actor(kind, id string) (*Actor, bool) {
	return p.node.scheduler.load(kind, id)
}

// 开始监听
func (p *Proxy) watch() {
	p.gateLinker.WatchUserLocate()

	p.gateLinker.WatchClusterInstance()

	p.nodeLinker.WatchUserLocate()

	p.nodeLinker.WatchClusterInstance()
}
