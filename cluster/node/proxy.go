package node

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport"
)

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
// 单个用户可以绑定到多个节点服务器上，相同名称的节点服务器只能绑定一个，多次绑定会到相同名称的节点服务器会覆盖之前的绑定。
// 绑定操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (p *Proxy) BindNode(ctx context.Context, uid int64, nameAndNID ...string) error {
	if len(nameAndNID) >= 2 && nameAndNID[0] != "" && nameAndNID[1] != "" {
		return p.link.BindNode(ctx, uid, nameAndNID[0], nameAndNID[1])
	} else {
		return p.link.BindNode(ctx, uid, p.node.opts.name, p.node.opts.id)
	}
}

// UnbindNode 解绑节点
// 解绑时会对对应名称的节点服务器进行解绑，解绑时会对解绑节点ID进行校验，不匹配则解绑失败。
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (p *Proxy) UnbindNode(ctx context.Context, uid int64, nameAndNID ...string) error {
	if len(nameAndNID) >= 2 && nameAndNID[0] != "" && nameAndNID[1] != "" {
		return p.link.UnbindNode(ctx, uid, nameAndNID[0], nameAndNID[1])
	} else {
		return p.link.UnbindNode(ctx, uid, p.node.opts.name, p.node.opts.id)
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
func (p *Proxy) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	return p.link.LocateNode(ctx, uid, name)
}

// AskNode 检测用户是否在给定的节点上
func (p *Proxy) AskNode(ctx context.Context, uid int64, name, nid string) (string, bool, error) {
	return p.link.AskNode(ctx, uid, name, nid)
}

// FetchGateList 拉取网关列表
func (p *Proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	list := make([]string, 0, len(states))
	for _, state := range states {
		list = append(list, state.String())
	}

	return p.link.FetchServiceList(ctx, cluster.Gate.String(), list...)
}

// FetchNodeList 拉取节点列表
func (p *Proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	list := make([]string, 0, len(states))
	for _, state := range states {
		list = append(list, state.String())
	}

	return p.link.FetchServiceList(ctx, cluster.Node.String(), list...)
}

// GetIP 获取客户端IP
func (p *Proxy) GetIP(ctx context.Context, args *cluster.GetIPArgs) (string, error) {
	return p.link.GetIP(ctx, args)
}

// Push 推送消息
func (p *Proxy) Push(ctx context.Context, args *cluster.PushArgs) error {
	return p.link.Push(ctx, args)
}

// Multicast 推送组播消息
func (p *Proxy) Multicast(ctx context.Context, args *cluster.MulticastArgs) (int64, error) {
	return p.link.Multicast(ctx, args)
}

// Broadcast 推送广播消息
func (p *Proxy) Broadcast(ctx context.Context, args *cluster.BroadcastArgs) (int64, error) {
	return p.link.Broadcast(ctx, args)
}

// Deliver 投递消息给节点处理
func (p *Proxy) Deliver(ctx context.Context, args *cluster.DeliverArgs) error {
	if args.NID != p.GetID() {
		return p.link.Deliver(ctx, &link.DeliverArgs{
			NID:     args.NID,
			UID:     args.UID,
			Message: args.Message,
		})
	} else {
		p.node.router.deliver("", args.NID, 0, args.UID, args.Message.Seq, args.Message.Route, args.Message.Data)
	}

	return nil
}

// Stat 统计会话总数
func (p *Proxy) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	return p.link.Stat(ctx, kind)
}

// Disconnect 断开连接
func (p *Proxy) Disconnect(ctx context.Context, args *cluster.DisconnectArgs) error {
	return p.link.Disconnect(ctx, args)
}

// Invoke 调用函数（线程安全）
func (p *Proxy) Invoke(fn func()) {
	p.node.fnChan <- fn
}

// 启动监听
func (p *Proxy) watch(ctx context.Context) {
	p.link.WatchUserLocate(ctx, cluster.Gate.String(), cluster.Node.String())

	p.link.WatchServiceInstance(ctx, cluster.Gate.String(), cluster.Node.String())
}
