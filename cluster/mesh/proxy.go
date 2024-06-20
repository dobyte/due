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

// AddServiceProvider 添加服务提供者
func (p *Proxy) AddServiceProvider(name string, desc interface{}, provider interface{}) {
	p.mesh.addServiceProvider(name, desc, provider)
}

// AddHookListener 添加钩子监听器
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.mesh.addHookListener(hook, handler)
}

// NewMeshClient 新建微服务客户端
// target参数可分为两种模式:
// 服务直连模式: 	direct://127.0.0.1:8011
// 服务发现模式: 	discovery://service_name
func (p *Proxy) NewMeshClient(target string) (transport.ServiceClient, error) {
	return p.mesh.opts.transporter.NewMeshClient(target)
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
func (p *Proxy) BindNode(ctx context.Context, uid int64, name, nid string) error {
	return p.nodeLinker.Bind(ctx, uid, name, nid)
}

// UnbindNode 解绑节点
// 解绑时会对对应名称的节点服务器进行解绑，解绑时会对解绑节点ID进行校验，不匹配则解绑失败。
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (p *Proxy) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	return p.nodeLinker.Unbind(ctx, uid, name, nid)
}

// LocateGate 定位用户所在网关
func (p *Proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	return p.gateLinker.Locate(ctx, uid)
}

// AskGate 检测用户是否在给定的网关上
func (p *Proxy) AskGate(ctx context.Context, gid string, uid int64) (string, bool, error) {
	return p.gateLinker.Ask(ctx, gid, uid)
}

// LocateNode 定位用户所在节点
func (p *Proxy) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	return p.nodeLinker.Locate(ctx, uid, name)
}

// AskNode 检测用户是否在给定的节点上
func (p *Proxy) AskNode(ctx context.Context, uid int64, name, nid string) (string, bool, error) {
	return p.nodeLinker.Ask(ctx, uid, name, nid)
}

// FetchGateList 拉取网关列表
func (p *Proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	//list := make([]string, 0, len(states))
	//for _, state := range states {
	//	list = append(list, state.String())
	//}
	//
	//return p.link.FetchServiceList(ctx, cluster.Gate.String(), list...)

	return nil, nil
}

// FetchNodeList 拉取节点列表
func (p *Proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	//list := make([]string, 0, len(states))
	//for _, state := range states {
	//	list = append(list, state.String())
	//}
	//
	//return p.link.FetchServiceList(ctx, cluster.Node.String(), list...)

	return nil, nil
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
