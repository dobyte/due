package mesh

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/transport"
)

type (
	GetIPArgs      = link.GetIPArgs
	PushArgs       = link.PushArgs
	MulticastArgs  = link.MulticastArgs
	BroadcastArgs  = link.BroadcastArgs
	DisconnectArgs = link.DisconnectArgs
	Message        = link.Message
)

type Proxy struct {
	mesh *Mesh
	link *link.Link // 链接
}

func newProxy(mesh *Mesh) *Proxy {
	return &Proxy{mesh: mesh, link: link.NewLink(&link.Options{
		Codec:       mesh.opts.codec,
		Locator:     mesh.opts.locator,
		Registry:    mesh.opts.registry,
		Encryptor:   mesh.opts.encryptor,
		Transporter: mesh.opts.transporter,
	})}
}

// AddServiceProvider 添加服务提供者
func (p *Proxy) AddServiceProvider(name string, desc interface{}, provider interface{}) {
	if p.mesh.state != cluster.Shut {
		log.Warnf("the mesh server is working, can't add server provider")
		return
	}

	p.mesh.services = append(p.mesh.services, &serviceEntity{
		name:     name,
		desc:     desc,
		provider: provider,
	})
}

// NewServiceClient 新建微服务客户端
// target参数可分为两种模式:
// 直连模式: 	direct://127.0.0.1:8011
// 服务发现模式: 	discovery://service_name
func (p *Proxy) NewServiceClient(target string) (transport.ServiceClient, error) {
	return p.mesh.opts.transporter.NewServiceClient(target)
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
func (p *Proxy) BindNode(ctx context.Context, uid int64, nid string) error {
	return p.link.BindNode(ctx, uid, nid)
}

// UnbindNode 解绑节点
// 解绑时会对解绑节点ID进行校验，不匹配则解绑失败
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上
func (p *Proxy) UnbindNode(ctx context.Context, uid int64, nid string) error {
	return p.link.UnbindNode(ctx, uid, nid)
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

// Disconnect 断开连接
func (p *Proxy) Disconnect(ctx context.Context, args *DisconnectArgs) error {
	return p.link.Disconnect(ctx, args)
}

// 启动监听
func (p *Proxy) watch(ctx context.Context) {
	p.link.WatchUserLocate(ctx, cluster.Gate)

	p.link.WatchServiceInstance(ctx, cluster.Gate, cluster.Node)
}
