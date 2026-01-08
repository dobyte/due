package mesh

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport"
)

type Proxy struct {
	mesh       *Mesh            // 微服务器
	gateLinker *link.GateLinker // 网关链接器
	nodeLinker *link.NodeLinker // 节点链接器
}

func newProxy(mesh *Mesh) *Proxy {
	opts := &link.Options{
		InsID:     mesh.opts.id,
		InsKind:   cluster.Mesh,
		Codec:     mesh.opts.codec,
		Locator:   mesh.opts.locator,
		Registry:  mesh.opts.registry,
		Encryptor: mesh.opts.encryptor,
	}

	return &Proxy{
		mesh:       mesh,
		gateLinker: link.NewGateLinker(mesh.opts.ctx, opts),
		nodeLinker: link.NewNodeLinker(mesh.opts.ctx, opts),
	}
}

// GetID 获取当前实例ID
func (p *Proxy) GetID() string {
	return p.mesh.opts.id
}

// GetName 获取当前实例名称
func (p *Proxy) GetName() string {
	return p.mesh.opts.name
}

// AddServiceProvider 添加服务提供者
func (p *Proxy) AddServiceProvider(name string, desc, provider any) {
	p.mesh.addServiceProvider(name, desc, provider)
}

// AddHookListener 添加钩子监听器
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.mesh.addHookListener(hook, handler)
}

// NewMeshClient 新建微服务客户端
// target参数可分为三种模式:
// 服务直连模式: 	direct://127.0.0.1:8011
// 服务直连模式: 	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
// 服务发现模式: 	discovery://service_name
func (p *Proxy) NewMeshClient(target string) (transport.Client, error) {
	return p.mesh.opts.transporter.NewClient(target)
}

// HasGate 检测是否存在某个网关
func (p *Proxy) HasGate(gid string) bool {
	return p.gateLinker.HasGate(gid)
}

// AskGate 检测用户是否在给定的网关上
func (p *Proxy) AskGate(ctx context.Context, gid string, uid int64) (string, bool, error) {
	return p.gateLinker.AskGate(ctx, gid, uid)
}

// LocateGate 定位用户所在网关
func (p *Proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	return p.gateLinker.LocateGate(ctx, uid)
}

// BindGate 绑定网关
func (p *Proxy) BindGate(ctx context.Context, gid string, cid, uid int64) error {
	return p.gateLinker.BindGate(ctx, gid, cid, uid)
}

// UnbindGate 解绑网关
func (p *Proxy) UnbindGate(ctx context.Context, uid int64) error {
	return p.gateLinker.UnbindGate(ctx, uid)
}

// FetchGateList 拉取网关列表
func (p *Proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.gateLinker.FetchGateList(ctx, states...)
}

// HasNode 检测是否存在某个节点
func (p *Proxy) HasNode(nid string) bool {
	return p.nodeLinker.HasNode(nid)
}

// AskNode 检测用户是否在给定的节点上
func (p *Proxy) AskNode(ctx context.Context, uid int64, name, nid string) (string, bool, error) {
	return p.nodeLinker.AskNode(ctx, uid, name, nid)
}

// LocateNode 定位用户所在节点
func (p *Proxy) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	return p.nodeLinker.LocateNode(ctx, uid, name)
}

// LocateNodes 定位用户所在节点列表
func (p *Proxy) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	return p.nodeLinker.LocateNodes(ctx, uid)
}

// BindNode 绑定节点
// 单个用户可以绑定到多个节点服务器上，相同名称的节点服务器只能绑定一个，多次绑定会到相同名称的节点服务器会覆盖之前的绑定。
// 绑定操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (p *Proxy) BindNode(ctx context.Context, uid int64, name, nid string) error {
	return p.nodeLinker.BindNode(ctx, uid, name, nid)
}

// UnbindNode 解绑节点
// 解绑时会对对应名称的节点服务器进行解绑，解绑时会对解绑节点ID进行校验，不匹配则解绑失败。
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (p *Proxy) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	return p.nodeLinker.UnbindNode(ctx, uid, name, nid)
}

// FetchNodeList 拉取节点列表
func (p *Proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.nodeLinker.FetchNodeList(ctx, states...)
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
// args.Ack设为true时可获得消息真实发送的情况
func (p *Proxy) Push(ctx context.Context, args *cluster.PushArgs) error {
	return p.gateLinker.Push(ctx, args)
}

// Multicast 推送组播消息
// 要想获得推送成功的目标数，需将args.Ack设为true
func (p *Proxy) Multicast(ctx context.Context, args *cluster.MulticastArgs) (int64, error) {
	return p.gateLinker.Multicast(ctx, args)
}

// Broadcast 推送广播消息
// 要想获得推送成功的目标数，需将args.Ack设为true
func (p *Proxy) Broadcast(ctx context.Context, args *cluster.BroadcastArgs) (int64, error) {
	return p.gateLinker.Broadcast(ctx, args)
}

// Publish 发布消息
// 要想获得推送成功的目标数，需将args.Ack设为true
func (p *Proxy) Publish(ctx context.Context, args *cluster.PublishArgs) (int64, error) {
	return p.gateLinker.Publish(ctx, args)
}

// Subscribe 订阅频道
func (p *Proxy) Subscribe(ctx context.Context, args *cluster.SubscribeArgs) error {
	return p.gateLinker.Subscribe(ctx, args)
}

// Unsubscribe 取消订阅频道
func (p *Proxy) Unsubscribe(ctx context.Context, args *cluster.UnsubscribeArgs) error {
	return p.gateLinker.Unsubscribe(ctx, args)
}

// Deliver 投递消息给节点处理
func (p *Proxy) Deliver(ctx context.Context, args *cluster.DeliverArgs) error {
	return p.nodeLinker.Deliver(ctx, &link.DeliverArgs{
		NID:    args.NID,
		UID:    args.UID,
		Route:  args.Message.Route,
		Buffer: args.Message,
	})
}

// 开始监听
func (p *Proxy) watch() {
	p.gateLinker.WatchUserLocate()

	p.gateLinker.WatchClusterInstance()

	p.nodeLinker.WatchUserLocate()

	p.nodeLinker.WatchClusterInstance()
}
