package node

import (
	"context"
	"sync"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/link"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/petermattis/goid"
)

type Proxy struct {
	node       *Node            // 节点服务器
	gateLinker *link.GateLinker // 网关链接器
	nodeLinker *link.NodeLinker // 节点链接器
}

func newProxy(node *Node) *Proxy {
	return &Proxy{
		node: node,
		gateLinker: link.NewGateLinker(node.ctx, &link.Options{
			ID:                  node.opts.id,
			Kind:                cluster.Node,
			Codec:               node.opts.codec,
			Locator:             node.opts.locator,
			Registry:            node.opts.registry,
			Encryptor:           node.opts.encryptor,
			ConnNum:             node.opts.linker.connNum,
			CallTimeout:         node.opts.linker.callTimeout,
			DialTimeout:         node.opts.linker.dialTimeout,
			DialRetryTimes:      node.opts.linker.dialRetryTimes,
			FaultRecoveryTime:   node.opts.linker.faultRecoveryTime,
			CommandQueueSize:    node.opts.linker.commandQueueSize,
			CommandWriteTimeout: node.opts.linker.commandWriteTimeout,
		}),
		nodeLinker: link.NewNodeLinker(node.ctx, &link.Options{
			ID:                  node.opts.id,
			Kind:                cluster.Node,
			Codec:               node.opts.codec,
			Locator:             node.opts.locator,
			Registry:            node.opts.registry,
			Encryptor:           node.opts.encryptor,
			ConnNum:             node.opts.linker.connNum,
			CallTimeout:         node.opts.linker.callTimeout,
			DialTimeout:         node.opts.linker.dialTimeout,
			DialRetryTimes:      node.opts.linker.dialRetryTimes,
			FaultRecoveryTime:   node.opts.linker.faultRecoveryTime,
			CommandQueueSize:    node.opts.linker.commandQueueSize,
			CommandWriteTimeout: node.opts.linker.commandWriteTimeout,
			WaitHandler:         node.doAddWait,
			DoneHandler:         node.doDoneWait,
		}),
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
	return p.node.setState(state)
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
func (p *Proxy) AddRouteHandler(route int32, handler RouteHandler, opts ...RouteOptions) {
	p.node.router.AddRouteHandler(route, handler, opts...)
}

// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
func (p *Proxy) SetDefaultRouteHandler(handler RouteHandler) {
	p.node.router.SetDefaultRouteHandler(handler)
}

// AddEventHandler 添加事件处理器
func (p *Proxy) AddEventHandler(event cluster.Event, handler EventHandler) {
	p.node.trigger.addEventHandler(event, handler)
}

// AddHookListener 添加钩子监听器
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.node.addHookListener(hook, handler)
}

// AddServiceProvider 添加服务提供者
func (p *Proxy) AddServiceProvider(name string, desc, provider any) {
	p.node.addServiceProvider(name, desc, provider)
}

// NewMeshClient 新建微服务客户端
// target参数可分为三种种模式:
// 服务直连模式: 	direct://127.0.0.1:8011
// 服务直连模式: 	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
// 服务发现模式: 	discovery://service_name
func (p *Proxy) NewMeshClient(target string) (transport.Client, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	}

	if p.node.opts.transporter == nil {
		return nil, errors.ErrMissingTransporter
	}

	return p.node.opts.transporter.NewClient(target)
}

// HasGate 检测是否存在某个网关
func (p *Proxy) HasGate(gid string) (bool, error) {
	if p.node.isShut() {
		return false, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.HasGate(gid), nil
	}
}

// AskGate 检测用户是否在给定的网关上
func (p *Proxy) AskGate(ctx context.Context, gid string, uid int64) (string, bool, error) {
	if p.node.isShut() {
		return "", false, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.AskGate(ctx, gid, uid)
	}
}

// LocateGate 定位用户所在网关
func (p *Proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	if p.node.isShut() {
		return "", errors.ErrNodeShutdown
	} else {
		return p.gateLinker.LocateGate(ctx, uid)
	}
}

// BindGate 绑定网关
func (p *Proxy) BindGate(ctx context.Context, gid string, cid, uid int64) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.BindGate(ctx, gid, cid, uid)
	}
}

// UnbindGate 解绑网关
func (p *Proxy) UnbindGate(ctx context.Context, uid int64) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.UnbindGate(ctx, uid)
	}
}

// FetchGateList 拉取网关列表
func (p *Proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.FetchGateList(ctx, states...)
	}
}

// HasNode 检测是否存在某个节点
func (p *Proxy) HasNode(nid string) bool {
	return p.nodeLinker.HasNode(nid)
}

// AskNode 检测用户是否在给定的节点上
func (p *Proxy) AskNode(ctx context.Context, uid int64, name, nid string) (string, bool, error) {
	if p.node.isShut() {
		return "", false, errors.ErrNodeShutdown
	} else {
		return p.nodeLinker.AskNode(ctx, uid, name, nid)
	}
}

// LocateNode 定位用户所在节点
func (p *Proxy) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	if p.node.isShut() {
		return "", errors.ErrNodeShutdown
	} else {
		return p.nodeLinker.LocateNode(ctx, uid, name)
	}
}

// LocateNodes 定位用户所在节点列表
func (p *Proxy) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	} else {
		return p.nodeLinker.LocateNodes(ctx, uid)
	}
}

// BindNode 绑定节点
// 单个用户可以绑定到多个节点服务器上，相同名称的节点服务器只能绑定一个，多次绑定会到相同名称的节点服务器会覆盖之前的绑定。
// 绑定操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (p *Proxy) BindNode(ctx context.Context, uid int64, nameAndNID ...string) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		name, nid := p.node.opts.name, p.node.opts.id

		if len(nameAndNID) >= 2 && nameAndNID[0] != "" && nameAndNID[1] != "" {
			name, nid = nameAndNID[0], nameAndNID[1]
		}

		return p.nodeLinker.BindNode(ctx, uid, name, nid)
	}
}

// UnbindNode 解绑节点
// 解绑时会对对应名称的节点服务器进行解绑，解绑时会对解绑节点ID进行校验，不匹配则解绑失败。
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (p *Proxy) UnbindNode(ctx context.Context, uid int64, nameAndNID ...string) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		name, nid := p.node.opts.name, p.node.opts.id

		if len(nameAndNID) >= 2 && nameAndNID[0] != "" && nameAndNID[1] != "" {
			name, nid = nameAndNID[0], nameAndNID[1]
		}

		return p.nodeLinker.UnbindNode(ctx, uid, name, nid)
	}
}

// FetchNodeList 拉取节点列表
func (p *Proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	} else {
		return p.nodeLinker.FetchNodeList(ctx, states...)
	}
}

// BindActor 绑定Actor
func (p *Proxy) BindActor(uid int64, kind, id string) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.node.scheduler.bindActor(uid, kind, id)
	}
}

// UnbindActor 解绑Actor
func (p *Proxy) UnbindActor(uid int64, kind string) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.node.scheduler.unbindActor(uid, kind)
	}
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
	if p.node.isShut() {
		return "", errors.ErrNodeShutdown
	} else {
		return p.gateLinker.GetIP(ctx, args)
	}
}

// Stat 统计会话总数
func (p *Proxy) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	if p.node.isShut() {
		return 0, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Stat(ctx, kind)
	}
}

// IsOnline 检测是否在线
func (p *Proxy) IsOnline(ctx context.Context, args *cluster.IsOnlineArgs) (bool, error) {
	if p.node.isShut() {
		return false, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.IsOnline(ctx, args)
	}
}

// Disconnect 断开连接
func (p *Proxy) Disconnect(ctx context.Context, args *cluster.DisconnectArgs) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Disconnect(ctx, args)
	}
}

// Push 推送消息
// args.Ack设为true时可获得消息真实发送的情况
func (p *Proxy) Push(ctx context.Context, args *cluster.PushArgs) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Push(ctx, args)
	}
}

// Multicast 推送组播消息
// 要想获得推送成功的目标数，需将args.Ack设为true
func (p *Proxy) Multicast(ctx context.Context, args *cluster.MulticastArgs) (int64, error) {
	if p.node.isShut() {
		return 0, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Multicast(ctx, args)
	}
}

// Broadcast 推送广播消息
// 要想获得推送成功的目标数，需将args.Ack设为true
func (p *Proxy) Broadcast(ctx context.Context, args *cluster.BroadcastArgs) (int64, error) {
	if p.node.isShut() {
		return 0, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Broadcast(ctx, args)
	}
}

// Publish 发布消息
// 要想获得推送成功的目标数，需将args.Ack设为true
func (p *Proxy) Publish(ctx context.Context, args *cluster.PublishArgs) (int64, error) {
	if p.node.isShut() {
		return 0, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Publish(ctx, args)
	}
}

// Subscribe 订阅频道
func (p *Proxy) Subscribe(ctx context.Context, args *cluster.SubscribeArgs) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Subscribe(ctx, args)
	}
}

// Unsubscribe 取消订阅频道
func (p *Proxy) Unsubscribe(ctx context.Context, args *cluster.UnsubscribeArgs) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Unsubscribe(ctx, args)
	}
}

// Deliver 投递消息给节点处理
func (p *Proxy) Deliver(ctx context.Context, args *cluster.DeliverArgs) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	}

	if args.NID == p.node.opts.id {
		return errors.ErrIllegalOperation
	}

	return p.nodeLinker.Deliver(ctx, &link.DeliverArgs{
		NID:    args.NID,
		UID:    args.UID,
		Route:  args.Message.Route,
		Buffer: args.Message,
	})
}

// Invoke 调用函数（线程安全）
func (p *Proxy) Invoke(f func(), isBlock ...bool) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	}

	if len(isBlock) > 0 && isBlock[0] {
		if p.node.dispatchGoid.Load() == goid.Get() {
			xcall.Call(f)
		} else {
			p.node.doAddWait()

			wg := &sync.WaitGroup{}
			wg.Add(1)

			if err := p.node.tasker.commit(func() {
				defer wg.Done()

				f()
			}); err != nil {
				p.node.doDoneWait()
				return err
			}

			wg.Wait()
		}
	} else {
		p.node.doAddWait()

		if err := p.node.tasker.commit(f); err != nil {
			p.node.doDoneWait()
			return err
		}
	}

	return nil
}

// AfterFunc 延迟调用，与官方的time.AfterFunc用法一致
func (p *Proxy) AfterFunc(d time.Duration, f func()) (*Timer, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	}

	p.node.doAddWait()

	timer := time.AfterFunc(d, func() {
		defer p.node.doDoneWait()

		if p.node.isShut() {
			log.Warnf("node after func failed: %v", errors.ErrNodeShutdown)
		} else {
			xcall.Call(f)
		}
	})

	return &Timer{node: p.node, timer: timer}, nil
}

// AfterInvoke 延迟调用（线程安全）
func (p *Proxy) AfterInvoke(d time.Duration, f func()) (*Timer, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	}

	p.node.doAddWait()

	timer := time.AfterFunc(d, func() {
		var err error

		if p.node.isShut() {
			err = errors.ErrNodeShutdown
		} else {
			err = p.node.tasker.commit(f)
		}

		if err != nil {
			log.Warnf("node after invoke failed: %v", err)
			p.node.doDoneWait()
		}
	})

	return &Timer{node: p.node, timer: timer}, nil
}

// Spawn 衍生出一个新的Actor
func (p *Proxy) Spawn(creator Creator, opts ...ActorOption) (*Actor, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	} else {
		return p.node.scheduler.spawn(creator, opts...)
	}
}

// Kill 杀死存在的一个Actor
func (p *Proxy) Kill(kind, id string) bool {
	if p.node.isShut() {
		return false
	} else {
		return p.node.scheduler.kill(kind, id)
	}
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
