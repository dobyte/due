package master

import (
	"context"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport"
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

// GetID 获取当前管理节点ID
func (p *Proxy) GetID() string {
	return p.master.opts.id
}

// GetName 获取当前管理节点名称
func (p *Proxy) GetName() string {
	return p.master.opts.name
}

// LoadConfig 加载配置
func (p *Proxy) LoadConfig(ctx context.Context, file string) ([]*config.Configuration, error) {
	if p.master.opts.configurator != nil {
		return p.master.opts.configurator.Load(ctx, p.master.opts.configSource.Name(), file)
	}

	return nil, errors.ErrNotFoundConfigSource
}

// StoreConfig 保存配置
func (p *Proxy) StoreConfig(ctx context.Context, file string, content interface{}) error {
	if p.master.opts.configurator != nil {
		return p.master.opts.configurator.Store(ctx, p.master.opts.configSource.Name(), file, content)
	}

	return errors.ErrNotFoundConfigSource
}

// AddHookListener 添加钩子监听器
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.master.addHookListener(hook, handler)
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
func (p *Proxy) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	return p.link.LocateNode(ctx, uid, name)
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
func (p *Proxy) GetIP(ctx context.Context, uid int64) (string, error) {
	return p.link.GetIP(ctx, &link.GetIPArgs{
		Kind:   session.User,
		Target: uid,
	})
}

// Push 推送消息
func (p *Proxy) Push(ctx context.Context, uid int64, message *cluster.Message) error {
	return p.link.Push(ctx, &link.PushArgs{
		Kind:    session.User,
		Target:  uid,
		Message: message,
	})
}

// Multicast 推送组播消息
func (p *Proxy) Multicast(ctx context.Context, uids []int64, message *cluster.Message) (int64, error) {
	return p.link.Multicast(ctx, &link.MulticastArgs{
		Kind:    session.User,
		Targets: uids[:],
		Message: message,
	})
}

// Broadcast 推送广播消息
func (p *Proxy) Broadcast(ctx context.Context, kind session.Kind, message *cluster.Message) (int64, error) {
	return p.link.Broadcast(ctx, &link.BroadcastArgs{
		Kind:    kind,
		Message: message,
	})
}

// Deliver 投递消息给节点处理
func (p *Proxy) Deliver(ctx context.Context, uid int64, message *cluster.Message) error {
	return p.link.Deliver(ctx, &link.DeliverArgs{
		UID:     uid,
		Message: message,
	})
}

// Stat 统计会话总数
func (p *Proxy) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	return p.link.Stat(ctx, kind)
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
	p.link.WatchUserLocate(ctx, cluster.Gate.String(), cluster.Node.String())

	p.link.WatchServiceInstance(ctx, cluster.Gate.String(), cluster.Node.String())
}
