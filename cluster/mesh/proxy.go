package mesh

import (
	"context"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport"
)

// Proxy 代理
// 提供微服务服务器对外可见的完整功能API，包括网关/节点链接与消息推送等
type Proxy struct {
	mesh       *Mesh            // 微服务器
	gateLinker *link.GateLinker // 网关链接器
	nodeLinker *link.NodeLinker // 节点链接器
}

// 创建微服务代理
// 初始化网关链接器与节点链接器，底层复用Mesh的编解码器、定位器、注册器等配置
// @param mesh *Mesh 微服务服务器
// @return @1 *Proxy 微服务代理
func newProxy(mesh *Mesh) *Proxy {
	opts := &link.Options{
		ID:                  mesh.opts.id,
		Kind:                cluster.Mesh,
		Codec:               mesh.opts.codec,
		Locator:             mesh.opts.locator,
		Registry:            mesh.opts.registry,
		Encryptor:           mesh.opts.encryptor,
		ConnNum:             mesh.opts.linker.connNum,
		CallTimeout:         mesh.opts.linker.callTimeout,
		DialTimeout:         mesh.opts.linker.dialTimeout,
		DialRetryTimes:      mesh.opts.linker.dialRetryTimes,
		FaultRecoveryTime:   mesh.opts.linker.faultRecoveryTime,
		CommandQueueSize:    mesh.opts.linker.commandQueueSize,
		CommandWriteTimeout: mesh.opts.linker.commandWriteTimeout,
	}

	return &Proxy{
		mesh:       mesh,
		gateLinker: link.NewGateLinker(mesh.ctx, opts),
		nodeLinker: link.NewNodeLinker(mesh.ctx, opts),
	}
}

// GetID 获取当前实例ID
// @return @1 string 当前实例ID
func (p *Proxy) GetID() string {
	return p.mesh.opts.id
}

// GetName 获取当前实例名称
// @return @1 string 当前实例名称
func (p *Proxy) GetName() string {
	return p.mesh.opts.name
}

// GetState 获取当前微服务状态
// @return @1 cluster.State 当前微服务状态
func (p *Proxy) GetState() cluster.State {
	return p.mesh.getState()
}

// SetState 设置当前微服务状态
// @param state cluster.State 目标状态
// @return @1 error 状态设置失败时返回的错误
func (p *Proxy) SetState(state cluster.State) error {
	return p.mesh.setState(state)
}

// AddServiceProvider 添加服务提供者
// @param name string 服务名称
// @param desc any 服务描述对象
// @param provider any 服务提供者
func (p *Proxy) AddServiceProvider(name string, desc, provider any) {
	p.mesh.addServiceProvider(name, desc, provider)
}

// AddHookListener 添加钩子监听器
// @param hook cluster.Hook 钩子类型
// @param handler HookHandler 钩子处理函数
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.mesh.addHookListener(hook, handler)
}

// NewMeshClient 新建微服务客户端
// target参数可分为三种模式:
// 服务直连模式: 	direct://127.0.0.1:8011
// 服务直连模式: 	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
// 服务发现模式: 	discovery://service_name
// @param target string 微服务目标地址
// @return @1 transport.Client 微服务客户端
// @return @2 error 微服务关闭时返回的错误
func (p *Proxy) NewMeshClient(target string) (transport.Client, error) {
	if p.mesh.isShut() {
		return nil, errors.ErrMeshShutdown
	} else {
		return p.mesh.opts.transporter.NewClient(target)
	}
}

// HasGate 检测是否存在某个网关
// @param gid string 网关ID
// @return @1 bool 网关是否存在
// @return @2 error 微服务关闭时返回的错误
func (p *Proxy) HasGate(gid string) (bool, error) {
	if p.mesh.isShut() {
		return false, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.HasGate(gid), nil
	}
}

// AskGate 检测用户是否在给定的网关上
// @param ctx context.Context 上下文
// @param gid string 网关ID
// @param uid int64 用户ID
// @return @1 string 用户实际所在的网关ID
// @return @2 bool 用户是否在给定的网关上
// @return @3 error 微服务关闭时返回的错误
func (p *Proxy) AskGate(ctx context.Context, gid string, uid int64) (string, bool, error) {
	if p.mesh.isShut() {
		return "", false, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.AskGate(ctx, gid, uid)
	}
}

// LocateGate 定位用户所在网关
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @return @1 string 用户所在的网关ID
// @return @2 error 微服务关闭时返回的错误
func (p *Proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	if p.mesh.isShut() {
		return "", errors.ErrMeshShutdown
	} else {
		return p.gateLinker.LocateGate(ctx, uid)
	}
}

// BindGate 绑定网关
// @param ctx context.Context 上下文
// @param gid string 网关ID
// @param cid int64 连接ID
// @param uid int64 用户ID，绑定后用户与该连接关联
// @return @1 error 微服务关闭或绑定失败时返回的错误
func (p *Proxy) BindGate(ctx context.Context, gid string, cid, uid int64) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.BindGate(ctx, gid, cid, uid)
	}
}

// UnbindGate 解绑网关
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @return @1 error 微服务关闭或解绑失败时返回的错误
func (p *Proxy) UnbindGate(ctx context.Context, uid int64) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.UnbindGate(ctx, uid)
	}
}

// FetchGateList 拉取网关列表
// @param ctx context.Context 上下文
// @param states ...cluster.State 状态过滤条件
// @return @1 []*registry.ServiceInstance 网关服务实例列表
// @return @2 error 微服务关闭时返回的错误
func (p *Proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	if p.mesh.isShut() {
		return nil, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.FetchGateList(ctx, states...)
	}
}

// HasNode 检测是否存在某个节点
// @param nid string 节点ID
// @return @1 bool 节点是否存在
// @return @2 error 微服务关闭时返回的错误
func (p *Proxy) HasNode(nid string) (bool, error) {
	if p.mesh.isShut() {
		return false, errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.HasNode(nid), nil
	}
}

// AskNode 检测用户是否在给定的节点上
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @param name string 节点名称
// @param nid string 节点ID
// @return @1 string 用户实际所在的节点ID
// @return @2 bool 用户是否在给定的节点上
// @return @3 error 微服务关闭时返回的错误
func (p *Proxy) AskNode(ctx context.Context, uid int64, name, nid string) (string, bool, error) {
	if p.mesh.isShut() {
		return "", false, errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.AskNode(ctx, uid, name, nid)
	}
}

// LocateNode 定位用户所在节点
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @param name string 节点名称
// @return @1 string 用户所在的节点ID
// @return @2 error 微服务关闭时返回的错误
func (p *Proxy) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	if p.mesh.isShut() {
		return "", errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.LocateNode(ctx, uid, name)
	}
}

// LocateNodes 定位用户所在节点列表
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @return @1 map[string]string 用户绑定的节点名称到节点ID的映射
// @return @2 error 微服务关闭时返回的错误
func (p *Proxy) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	if p.mesh.isShut() {
		return nil, errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.LocateNodes(ctx, uid)
	}
}

// BindNode 绑定节点
// 单个用户可以绑定到多个节点服务器上，相同名称的节点服务器只能绑定一个，多次绑定会到相同名称的节点服务器会覆盖之前的绑定。
// 绑定操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @param name string 节点名称
// @param nid string 节点ID
// @return @1 error 微服务关闭或绑定失败时返回的错误
func (p *Proxy) BindNode(ctx context.Context, uid int64, name, nid string) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.BindNode(ctx, uid, name, nid)
	}
}

// UnbindNode 解绑节点
// 解绑时会对对应名称的节点服务器进行解绑，解绑时会对解绑节点ID进行校验，不匹配则解绑失败。
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
// @param ctx context.Context 上下文
// @param uid int64 用户ID
// @param name string 节点名称
// @param nid string 节点ID
// @return @1 error 微服务关闭或解绑失败时返回的错误
func (p *Proxy) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.UnbindNode(ctx, uid, name, nid)
	}
}

// FetchNodeList 拉取节点列表
// @param ctx context.Context 上下文
// @param states ...cluster.State 状态过滤条件
// @return @1 []*registry.ServiceInstance 节点服务实例列表
// @return @2 error 微服务关闭时返回的错误
func (p *Proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	if p.mesh.isShut() {
		return nil, errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.FetchNodeList(ctx, states...)
	}
}

// PackMessage 打包消息
// @param message *cluster.Message 待打包的消息
// @return @1 []byte 打包后的消息字节
// @return @2 error 打包失败时返回的错误
func (p *Proxy) PackMessage(message *cluster.Message) ([]byte, error) {
	buf, err := p.gateLinker.PackMessage(message, true)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// PackBuffer 打包Buffer
// @param message any 待打包的消息内容
// @return @1 []byte 打包后的消息字节
// @return @2 error 打包失败时返回的错误
func (p *Proxy) PackBuffer(message any) ([]byte, error) {
	return p.gateLinker.PackBuffer(message, true)
}

// GetIP 获取客户端IP
// @param ctx context.Context 上下文
// @param args *cluster.GetIPArgs 查询参数
// @return @1 string 客户端IP
// @return @2 error 微服务关闭或查询失败时返回的错误
func (p *Proxy) GetIP(ctx context.Context, args *cluster.GetIPArgs) (string, error) {
	if p.mesh.isShut() {
		return "", errors.ErrMeshShutdown
	} else {
		return p.gateLinker.GetIP(ctx, args)
	}
}

// Stat 统计会话总数
// @param ctx context.Context 上下文
// @param kind session.Kind 会话类型
// @return @1 int64 会话总数
// @return @2 error 微服务关闭时返回的错误
func (p *Proxy) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	if p.mesh.isShut() {
		return 0, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Stat(ctx, kind)
	}
}

// IsOnline 检测是否在线
// @param ctx context.Context 上下文
// @param args *cluster.IsOnlineArgs 查询参数
// @return @1 bool 是否在线
// @return @2 error 微服务关闭或查询失败时返回的错误
func (p *Proxy) IsOnline(ctx context.Context, args *cluster.IsOnlineArgs) (bool, error) {
	if p.mesh.isShut() {
		return false, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.IsOnline(ctx, args)
	}
}

// Disconnect 断开连接
// @param ctx context.Context 上下文
// @param args *cluster.DisconnectArgs 断开参数
// @return @1 error 微服务关闭或断开失败时返回的错误
func (p *Proxy) Disconnect(ctx context.Context, args *cluster.DisconnectArgs) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Disconnect(ctx, args)
	}
}

// Push 推送消息
// args.Ack设为true时可获得消息真实发送的情况
// @param ctx context.Context 上下文
// @param args *cluster.PushArgs 推送参数
// @return @1 error 微服务关闭或推送失败时返回的错误
func (p *Proxy) Push(ctx context.Context, args *cluster.PushArgs) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Push(ctx, args)
	}
}

// Multicast 推送组播消息
// 要想获得推送成功的目标数，需将args.Ack设为true
// @param ctx context.Context 上下文
// @param args *cluster.MulticastArgs 组播参数
// @return @1 int64 组播成功的目标数
// @return @2 error 微服务关闭或推送失败时返回的错误
func (p *Proxy) Multicast(ctx context.Context, args *cluster.MulticastArgs) (int64, error) {
	if p.mesh.isShut() {
		return 0, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Multicast(ctx, args)
	}
}

// Broadcast 推送广播消息
// 要想获得推送成功的目标数，需将args.Ack设为true
// @param ctx context.Context 上下文
// @param args *cluster.BroadcastArgs 广播参数
// @return @1 int64 广播成功的目标数
// @return @2 error 微服务关闭或推送失败时返回的错误
func (p *Proxy) Broadcast(ctx context.Context, args *cluster.BroadcastArgs) (int64, error) {
	if p.mesh.isShut() {
		return 0, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Broadcast(ctx, args)
	}
}

// Publish 发布消息
// 要想获得推送成功的目标数，需将args.Ack设为true
// @param ctx context.Context 上下文
// @param args *cluster.PublishArgs 发布参数
// @return @1 int64 发布成功的目标数
// @return @2 error 微服务关闭或发布失败时返回的错误
func (p *Proxy) Publish(ctx context.Context, args *cluster.PublishArgs) (int64, error) {
	if p.mesh.isShut() {
		return 0, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Publish(ctx, args)
	}
}

// Subscribe 订阅频道
// @param ctx context.Context 上下文
// @param args *cluster.SubscribeArgs 订阅参数
// @return @1 error 微服务关闭或订阅失败时返回的错误
func (p *Proxy) Subscribe(ctx context.Context, args *cluster.SubscribeArgs) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Subscribe(ctx, args)
	}
}

// Unsubscribe 取消订阅频道
// @param ctx context.Context 上下文
// @param args *cluster.UnsubscribeArgs 取消订阅参数
// @return @1 error 微服务关闭或取消订阅失败时返回的错误
func (p *Proxy) Unsubscribe(ctx context.Context, args *cluster.UnsubscribeArgs) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Unsubscribe(ctx, args)
	}
}

// Deliver 投递消息给节点处理
// @param ctx context.Context 上下文
// @param args *cluster.DeliverArgs 投递参数
// @return @1 error 微服务关闭或投递失败时返回的错误
func (p *Proxy) Deliver(ctx context.Context, args *cluster.DeliverArgs) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.Deliver(ctx, &link.DeliverArgs{
			NID:    args.NID,
			UID:    args.UID,
			Route:  args.Message.Route,
			Buffer: args.Message,
		})
	}
}

// 开始监听
// 监听用户定位与集群实例变化，网关与节点链接器均订阅相关变更
func (p *Proxy) watch() {
	p.gateLinker.WatchUserLocate()

	p.gateLinker.WatchClusterInstance()

	p.nodeLinker.WatchUserLocate()

	p.nodeLinker.WatchClusterInstance()
}
