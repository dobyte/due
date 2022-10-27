package node

import (
	"context"
	"errors"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/locate"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/transport"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/session"
	"golang.org/x/sync/errgroup"
)

var (
	ErrInvalidGID         = errors.New("invalid gate id")
	ErrInvalidNID         = errors.New("invalid node id")
	ErrInvalidSessionKind = errors.New("invalid session kind")
	ErrNotFoundUserSource = errors.New("not found user source")
	ErrReceiveTargetEmpty = errors.New("the receive target is empty")
	ErrUnableLocateSource = errors.New("unable to locate source")
)

type Proxy interface {
	// GetNID 获取当前节点ID
	GetNID() string
	// AddRouteHandler 添加路由处理器
	AddRouteHandler(route int32, encrypt, stateful bool, handler RouteHandler)
	// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
	SetDefaultRouteHandler(handler RouteHandler)
	// AddEventListener 添加事件监听器
	AddEventListener(event cluster.Event, handler EventHandler)
	// BindGate 绑定网关
	BindGate(ctx context.Context, gid string, cid, uid int64) error
	// UnbindGate 绑定网关
	UnbindGate(ctx context.Context, uid int64) error
	// BindNode 绑定节点
	BindNode(ctx context.Context, uid int64, nid ...string) error
	// UnbindNode 解绑节点
	UnbindNode(ctx context.Context, uid int64, nid ...string) error
	// LocateGate 定位用户所在网关
	LocateGate(ctx context.Context, uid int64) (string, error)
	// LocateNode 定位用户所在节点
	LocateNode(ctx context.Context, uid int64) (string, error)
	// FetchGateList 拉取网关列表
	FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error)
	// FetchNodeList 拉取节点列表
	FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error)
	// GetIP 获取客户端IP
	GetIP(ctx context.Context, args *GetIPArgs) (string, error)
	// Push 推送消息
	Push(ctx context.Context, args *PushArgs) error
	// Response 响应消息
	Response(ctx context.Context, req Request, message interface{}) error
	// Multicast 推送组播消息
	Multicast(ctx context.Context, args *MulticastArgs) (int64, error)
	// Broadcast 推送广播消息
	Broadcast(ctx context.Context, args *BroadcastArgs) (int64, error)
	// Disconnect 断开连接
	Disconnect(ctx context.Context, args *DisconnectArgs) error
	// Deliver 投递消息给节点处理
	Deliver(ctx context.Context, args *DeliverArgs) error
}

type proxy struct {
	node       *Node    // 节点
	sourceGate sync.Map // 用户来源网关
	sourceNode sync.Map // 用户来源节点
}

func newProxy(node *Node) *proxy {
	return &proxy{node: node}
}

// GetNID 获取当前节点ID
func (p *proxy) GetNID() string {
	return p.node.opts.id
}

// AddRouteHandler 添加路由处理器
func (p *proxy) AddRouteHandler(route int32, encrypt, stateful bool, handler RouteHandler) {
	p.node.addRouteHandler(route, encrypt, stateful, handler)
}

// SetDefaultRouteHandler 设置默认路由处理器，所有未注册的路由均走默认路由处理器
func (p *proxy) SetDefaultRouteHandler(handler RouteHandler) {
	p.node.defaultRouteHandler = handler
}

// AddEventListener 添加事件监听器
func (p *proxy) AddEventListener(event cluster.Event, handler EventHandler) {
	p.node.addEventListener(event, handler)
}

// BindGate 绑定网关
func (p *proxy) BindGate(ctx context.Context, gid string, cid, uid int64) error {
	client, err := p.getGateClientByGID(gid)
	if err != nil {
		return err
	}

	_, err = client.Bind(ctx, cid, uid)
	if err != nil {
		return err
	}

	p.sourceGate.Store(uid, gid)

	return nil
}

// UnbindGate 解绑网关
func (p *proxy) UnbindGate(ctx context.Context, uid int64) error {
	_, err := p.doGateRPC(ctx, uid, func(client transport.GateClient) (bool, interface{}, error) {
		miss, err := client.Unbind(ctx, uid)
		return miss, nil, err
	})
	if err != nil {
		return err
	}

	p.sourceGate.Delete(uid)

	return nil
}

// BindNode 绑定节点
// 单个用户只能被绑定到某一台节点服务器上，多次绑定会直接覆盖上次绑定
// 绑定操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上
// nid 为需要绑定的节点ID，默认绑定到当前节点上
func (p *proxy) BindNode(ctx context.Context, uid int64, nid ...string) error {
	if len(nid) == 0 || nid[0] == "" {
		nid = append(nid, p.node.opts.id)
	}

	err := p.node.opts.locator.Set(ctx, uid, cluster.Node, nid[0])
	if err != nil {
		return err
	}

	p.sourceNode.Store(uid, nid[0])

	return nil
}

// UnbindNode 解绑节点
// 解绑时会对解绑节点ID进行校验，不匹配则解绑失败
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上
// nid 为需要解绑的节点ID，默认解绑当前节点
func (p *proxy) UnbindNode(ctx context.Context, uid int64, nid ...string) error {
	if len(nid) == 0 || nid[0] == "" {
		nid = append(nid, p.node.opts.id)
	}

	err := p.node.opts.locator.Rem(ctx, uid, cluster.Node, nid[0])
	if err != nil {
		return err
	}

	p.sourceNode.Delete(uid)

	return nil
}

// LocateGate 定位用户所在网关
func (p *proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	if val, ok := p.sourceGate.Load(uid); ok {
		if insID := val.(string); insID != "" {
			return insID, nil
		}
	}

	gid, err := p.node.opts.locator.Get(ctx, uid, cluster.Gate)
	if err != nil {
		return "", err
	}

	if gid == "" {
		return "", ErrNotFoundUserSource
	}

	p.sourceGate.Store(uid, gid)

	return gid, nil
}

// LocateNode 定位用户所在节点
func (p *proxy) LocateNode(ctx context.Context, uid int64) (string, error) {
	if val, ok := p.sourceNode.Load(uid); ok {
		if nid := val.(string); nid != "" {
			return nid, nil
		}
	}

	nid, err := p.node.opts.locator.Get(ctx, uid, cluster.Node)
	if err != nil {
		return "", err
	}

	if nid == "" {
		return "", ErrNotFoundUserSource
	}

	p.sourceNode.Store(uid, nid)

	return nid, nil
}

// FetchGateList 拉取网关列表
func (p *proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.fetchInstanceList(ctx, cluster.Gate, states...)
}

// FetchNodeList 拉取节点列表
func (p *proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	return p.fetchInstanceList(ctx, cluster.Node, states...)
}

// 拉取实例列表
func (p *proxy) fetchInstanceList(ctx context.Context, kind cluster.Kind, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	services, err := p.node.opts.registry.Services(ctx, string(kind))
	if err != nil {
		return nil, err
	}

	if len(states) == 0 {
		return services, nil
	}

	mp := make(map[cluster.State]struct{}, len(states))
	for _, state := range states {
		mp[state] = struct{}{}
	}

	list := make([]*registry.ServiceInstance, 0, len(services))
	for i := range services {
		if _, ok := mp[services[i].State]; ok {
			list = append(list, services[i])
		}
	}

	return list, nil
}

// GetIP 获取客户端IP
func (p *proxy) GetIP(ctx context.Context, args *GetIPArgs) (string, error) {
	switch args.Kind {
	case session.Conn:
		return p.directGetIP(ctx, args.GID, args.Kind, args.Target)
	case session.User:
		if args.GID == "" {
			return p.indirectGetIP(ctx, args.Target)
		} else {
			return p.directGetIP(ctx, args.GID, args.Kind, args.Target)
		}
	default:
		return "", ErrInvalidSessionKind
	}
}

// 直接获取IP
func (p *proxy) directGetIP(ctx context.Context, gid string, kind session.Kind, target int64) (string, error) {
	client, err := p.getGateClientByGID(gid)
	if err != nil {
		return "", err
	}

	ip, _, err := client.GetIP(ctx, kind, target)
	return ip, err
}

// 间接获取IP
func (p *proxy) indirectGetIP(ctx context.Context, uid int64) (string, error) {
	v, err := p.doGateRPC(ctx, uid, func(client transport.GateClient) (bool, interface{}, error) {
		ip, miss, err := client.GetIP(ctx, session.User, uid)
		return miss, ip, err
	})
	if err != nil {
		return "", err
	}

	return v.(string), nil
}

// Push 推送消息
func (p *proxy) Push(ctx context.Context, args *PushArgs) error {
	switch args.Kind {
	case session.Conn:
		return p.directPush(ctx, args)
	case session.User:
		if args.GID == "" {
			return p.indirectPush(ctx, args)
		} else {
			return p.directPush(ctx, args)
		}
	default:
		return ErrInvalidSessionKind
	}
}

// 直接推送
func (p *proxy) directPush(ctx context.Context, args *PushArgs) error {
	buffer, err := p.toBuffer(args.Message.Data, args.Encrypt)
	if err != nil {
		return err
	}

	client, err := p.getGateClientByGID(args.GID)
	if err != nil {
		return err
	}

	_, err = client.Push(ctx, args.Kind, args.Target, &transport.Message{
		Seq:    args.Message.Seq,
		Route:  args.Message.Route,
		Buffer: buffer,
	})
	return err
}

// 间接推送
func (p *proxy) indirectPush(ctx context.Context, args *PushArgs) error {
	buffer, err := p.toBuffer(args.Message.Data, args.Encrypt)
	if err != nil {
		return err
	}

	_, err = p.doGateRPC(ctx, args.Target, func(client transport.GateClient) (bool, interface{}, error) {
		miss, err := client.Push(ctx, session.User, args.Target, &transport.Message{
			Seq:    args.Message.Seq,
			Route:  args.Message.Route,
			Buffer: buffer,
		})
		return miss, nil, err
	})

	return err
}

// Response 响应消息
func (p *proxy) Response(ctx context.Context, req Request, message interface{}) error {
	switch {
	case req.GID() != "":
		return p.directPush(ctx, &PushArgs{
			GID:     req.GID(),
			Kind:    session.Conn,
			Target:  req.CID(),
			Encrypt: p.node.checkRouteEncrypt(req.Route()),
			Message: &Message{
				Seq:   req.Seq(),
				Route: req.Route(),
				Data:  message,
			},
		})
	case req.NID() != "":
		return p.directDeliver(ctx, &DeliverArgs{
			NID: req.NID(),
			UID: req.UID(),
			Message: &Message{
				Seq:   req.Seq(),
				Route: req.Route(),
				Data:  message,
			},
		})
	default:
		return ErrUnableLocateSource
	}
}

// Multicast 推送组播消息
func (p *proxy) Multicast(ctx context.Context, args *MulticastArgs) (int64, error) {
	switch args.Kind {
	case session.Conn:
		return p.directMulticast(ctx, args)
	case session.User:
		if args.GID == "" {
			return p.indirectMulticast(ctx, args)
		} else {
			return p.directMulticast(ctx, args)
		}
	default:
		return 0, ErrInvalidSessionKind
	}
}

// 直接推送组播消息，只能推送到同一个网关服务器上
func (p *proxy) directMulticast(ctx context.Context, args *MulticastArgs) (int64, error) {
	if len(args.Targets) == 0 {
		return 0, ErrReceiveTargetEmpty
	}

	buffer, err := p.toBuffer(args.Message.Data, args.Encrypt)
	if err != nil {
		return 0, err
	}

	client, err := p.getGateClientByGID(args.GID)
	if err != nil {
		return 0, err
	}

	return client.Multicast(ctx, args.Kind, args.Targets, &transport.Message{
		Seq:    args.Message.Seq,
		Route:  args.Message.Route,
		Buffer: buffer,
	})
}

// 间接推送组播消息
func (p *proxy) indirectMulticast(ctx context.Context, args *MulticastArgs) (int64, error) {
	buffer, err := p.toBuffer(args.Message.Data, args.Encrypt)
	if err != nil {
		return 0, err
	}

	total := int64(0)
	eg, ctx := errgroup.WithContext(ctx)
	for _, target := range args.Targets {
		func(target int64) {
			eg.Go(func() error {
				_, err := p.doGateRPC(ctx, target, func(client transport.GateClient) (bool, interface{}, error) {
					miss, err := client.Push(ctx, session.User, target, &transport.Message{
						Seq:    args.Message.Seq,
						Route:  args.Message.Route,
						Buffer: buffer,
					})
					return miss, nil, err
				})
				if err != nil {
					return err
				}

				atomic.AddInt64(&total, 1)
				return nil
			})
		}(target)
	}

	err = eg.Wait()

	if total > 0 {
		return total, nil
	}

	return 0, err
}

// Broadcast 推送广播消息
func (p *proxy) Broadcast(ctx context.Context, args *BroadcastArgs) (int64, error) {
	buffer, err := p.toBuffer(args.Message.Data, args.Encrypt)
	if err != nil {
		return 0, err
	}

	total := int64(0)
	eg, ctx := errgroup.WithContext(ctx)
	p.node.router.RangeGateEndpoint(func(insID string, ep *router.Endpoint) bool {
		eg.Go(func() error {
			client, err := p.node.opts.transporter.NewGateClient(ep)
			if err != nil {
				return err
			}

			n, err := client.Broadcast(ctx, args.Kind, &transport.Message{
				Seq:    args.Message.Seq,
				Route:  args.Message.Route,
				Buffer: buffer,
			})
			if err != nil {
				return err
			}

			atomic.AddInt64(&total, n)

			return nil
		})

		return true
	})

	err = eg.Wait()

	if total > 0 {
		return total, nil
	}

	return total, err
}

// Disconnect 断开连接
func (p *proxy) Disconnect(ctx context.Context, args *DisconnectArgs) error {
	switch args.Kind {
	case session.Conn:
		return p.directDisconnect(ctx, args.GID, args.Kind, args.Target, args.IsForce)
	case session.User:
		if args.GID == "" {
			return p.indirectDisconnect(ctx, args.Target, args.IsForce)
		} else {
			return p.directDisconnect(ctx, args.GID, args.Kind, args.Target, args.IsForce)
		}
	default:
		return ErrInvalidSessionKind
	}
}

// 直接断开连接
func (p *proxy) directDisconnect(ctx context.Context, gid string, kind session.Kind, target int64, isForce bool) error {
	client, err := p.getGateClientByGID(gid)
	if err != nil {
		return err
	}

	_, err = client.Disconnect(ctx, kind, target, isForce)
	return err
}

// 间接断开连接
func (p *proxy) indirectDisconnect(ctx context.Context, uid int64, isForce bool) error {
	_, err := p.doGateRPC(ctx, uid, func(client transport.GateClient) (bool, interface{}, error) {
		miss, err := client.Disconnect(ctx, session.User, uid, isForce)
		return miss, nil, err
	})

	return err
}

// Deliver 投递消息给节点处理
func (p *proxy) Deliver(ctx context.Context, args *DeliverArgs) error {
	switch {
	case args.NID == p.node.opts.id:
		p.node.deliverRequest(&request{
			nid:   p.node.opts.id,
			uid:   args.UID,
			node:  p.node,
			seq:   args.Message.Seq,
			route: args.Message.Route,
			data:  args.Message.Data,
		})
		return nil
	case args.NID != "":
		return p.directDeliver(ctx, args)
	default:
		return p.indirectDeliver(ctx, args)
	}
}

// 直接投递
func (p *proxy) directDeliver(ctx context.Context, args *DeliverArgs) error {
	buffer, err := p.toBuffer(args.Message.Data, false)
	if err != nil {
		return err
	}

	client, err := p.getNodeClientByNID(args.NID)
	if err != nil {
		return err
	}

	_, err = client.Deliver(ctx, "", p.node.opts.id, 0, args.UID, &transport.Message{
		Seq:    args.Message.Seq,
		Route:  args.Message.Route,
		Buffer: buffer,
	})

	return err
}

// 间接投递
func (p *proxy) indirectDeliver(ctx context.Context, args *DeliverArgs) error {
	buffer, err := p.toBuffer(args.Message.Data, false)
	if err != nil {
		return err
	}

	_, err = p.doNodeRPC(ctx, args.Message.Route, args.UID, func(ctx context.Context, client transport.NodeClient) (bool, interface{}, error) {
		miss, err := client.Deliver(ctx, "", p.node.opts.id, 0, args.UID, &transport.Message{
			Seq:    args.Message.Seq,
			Route:  args.Message.Route,
			Buffer: buffer,
		})
		return miss, nil, err
	})

	return err
}

// 消息转buffer
func (p *proxy) toBuffer(message interface{}, encrypt bool) ([]byte, error) {
	if v, ok := message.([]byte); ok {
		return v, nil
	}

	buf, err := p.node.opts.codec.Marshal(message)
	if err != nil {
		return nil, err
	}

	if !encrypt {
		return buf, nil
	}

	return p.node.opts.encryptor.Encrypt(buf)
}

// 执行RPC调用
func (p *proxy) doGateRPC(ctx context.Context, uid int64, fn func(client transport.GateClient) (bool, interface{}, error)) (interface{}, error) {
	var (
		err       error
		gid       string
		lastGID   string
		client    transport.GateClient
		continued bool
		reply     interface{}
	)

	for i := 0; i < 2; i++ {
		if gid, err = p.LocateGate(ctx, uid); err != nil {
			return nil, err
		}

		if gid == lastGID {
			return reply, err
		}

		lastGID = gid

		client, err = p.getGateClientByGID(gid)
		if err != nil {
			return nil, err
		}

		continued, reply, err = fn(client)
		if continued {
			p.sourceGate.Delete(uid)
			continue
		}

		break
	}

	return reply, err
}

// 执行RPC调用
func (p *proxy) doNodeRPC(ctx context.Context, route int32, uid int64, fn func(ctx context.Context, client transport.NodeClient) (bool, interface{}, error)) (interface{}, error) {
	var (
		err       error
		nid       string
		prev      string
		client    transport.NodeClient
		entity    *router.Route
		ep        *router.Endpoint
		continued bool
		reply     interface{}
	)

	if entity, err = p.node.router.FindNodeRoute(route); err != nil {
		return nil, err
	}

	for i := 0; i < 2; i++ {
		if entity.Stateful() {
			if nid, err = p.LocateNode(ctx, uid); err != nil {
				return nil, err
			}
			if nid == prev {
				return reply, err
			}
			prev = nid
		}

		ep, err = entity.FindEndpoint(nid)
		if err != nil {
			return nil, err
		}

		client, err = p.node.opts.transporter.NewNodeClient(ep)
		if err != nil {
			return nil, err
		}

		continued, reply, err = fn(ctx, client)
		if continued {
			p.sourceNode.Delete(uid)
			continue
		}

		break
	}

	return reply, err
}

// 根据实例ID获取网关客户端
func (p *proxy) getGateClientByGID(gid string) (transport.GateClient, error) {
	if gid == "" {
		return nil, ErrInvalidGID
	}

	ep, err := p.node.router.FindGateEndpoint(gid)
	if err != nil {
		return nil, err
	}

	return p.node.opts.transporter.NewGateClient(ep)
}

// 根据实例ID获取节点客户端
func (p *proxy) getNodeClientByNID(nid string) (transport.NodeClient, error) {
	if nid == "" {
		return nil, ErrInvalidNID
	}

	ep, err := p.node.router.FindNodeEndpoint(nid)
	if err != nil {
		return nil, err
	}

	return p.node.opts.transporter.NewNodeClient(ep)
}

// 启动代理
func (p *proxy) watch(ctx context.Context) {
	watcher, err := p.node.opts.locator.Watch(ctx, cluster.Gate, cluster.Node)
	if err != nil {
		log.Fatalf("user locate event watch failed: %v", err)
	}

	go func() {
		defer watcher.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// exec watch
			}
			events, err := watcher.Next()
			if err != nil {
				continue
			}
			for _, event := range events {
				var source *sync.Map
				switch event.InsKind {
				case cluster.Gate:
					source = &p.sourceGate
				case cluster.Node:
					source = &p.sourceNode
				}

				if source == nil {
					continue
				}

				switch event.Type {
				case locate.SetLocation:
					source.Store(event.UID, event.InsID)
				case locate.RemLocation:
					source.Delete(event.UID)
				}
			}
		}
	}()
}
