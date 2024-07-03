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
	master     *Master          // 管理服务器
	gateLinker *link.GateLinker // 网关链接器
	nodeLinker *link.NodeLinker // 节点链接器
}

func newProxy(master *Master) *Proxy {
	opts := &link.Options{
		InsID:     master.opts.id,
		InsKind:   cluster.Master,
		Codec:     master.opts.codec,
		Locator:   master.opts.locator,
		Registry:  master.opts.registry,
		Encryptor: master.opts.encryptor,
	}

	return &Proxy{
		master:     master,
		gateLinker: link.NewGateLinker(master.opts.ctx, opts),
		nodeLinker: link.NewNodeLinker(master.opts.ctx, opts),
	}
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

// NewMeshClient 新建微服务客户端
// target参数可分为两种模式:
// 服务直连模式: 	direct://127.0.0.1:8011
// 服务发现模式: 	discovery://service_name
func (p *Proxy) NewMeshClient(target string) (transport.Client, error) {
	return p.master.opts.transporter.NewClient(target)
}

// LocateGate 定位用户所在网关
func (p *Proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	return p.gateLinker.Locate(ctx, uid)
}

// LocateNode 定位用户所在节点
func (p *Proxy) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	return p.nodeLinker.Locate(ctx, uid, name)
}

// FetchGateList 拉取网关列表
func (p *Proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.gateLinker.FetchGateList(ctx, states...)
}

// FetchNodeList 拉取节点列表
func (p *Proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.nodeLinker.FetchNodeList(ctx, states...)
}

// GetGateState 获取网关状态
func (p *Proxy) GetGateState(ctx context.Context, gid string) (cluster.State, error) {
	return p.gateLinker.GetState(ctx, gid)
}

// SetGateState 设置网关状态
func (p *Proxy) SetGateState(ctx context.Context, gid string, state cluster.State) error {
	return p.gateLinker.SetState(ctx, gid, state)
}

// GetNodeState 获取节点状态
func (p *Proxy) GetNodeState(ctx context.Context, nid string) (cluster.State, error) {
	return p.nodeLinker.GetState(ctx, nid)
}

// SetNodeState 设置节点状态
func (p *Proxy) SetNodeState(ctx context.Context, nid string, state cluster.State) error {
	return p.nodeLinker.SetState(ctx, nid, state)
}

// GetIP 获取客户端IP
func (p *Proxy) GetIP(ctx context.Context, uid int64) (string, error) {
	return p.gateLinker.GetIP(ctx, &cluster.GetIPArgs{
		Kind:   session.User,
		Target: uid,
	})
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
func (p *Proxy) Push(ctx context.Context, uid int64, message *cluster.Message) error {
	return p.gateLinker.Push(ctx, &cluster.PushArgs{
		Kind:    session.User,
		Target:  uid,
		Message: message,
	})
}

// Multicast 推送组播消息
func (p *Proxy) Multicast(ctx context.Context, uids []int64, message *cluster.Message) error {
	return p.gateLinker.Multicast(ctx, &cluster.MulticastArgs{
		Kind:    session.User,
		Targets: uids,
		Message: message,
	})
}

// Broadcast 推送广播消息
func (p *Proxy) Broadcast(ctx context.Context, kind session.Kind, message *cluster.Message) error {
	return p.gateLinker.Broadcast(ctx, &cluster.BroadcastArgs{
		Kind:    kind,
		Message: message,
	})
}

// 开始监听
func (p *Proxy) watch() {
	p.gateLinker.WatchUserLocate()

	p.gateLinker.WatchClusterInstance()

	p.nodeLinker.WatchUserLocate()

	p.nodeLinker.WatchClusterInstance()
}
