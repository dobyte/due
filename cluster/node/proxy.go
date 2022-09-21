package node

import (
	"context"
	"errors"
	"github.com/dobyte/due/cluster"
	"github.com/dobyte/due/locator"
	"github.com/dobyte/due/registry"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/dobyte/due/cluster/internal/code"
	"github.com/dobyte/due/cluster/internal/pb"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/router"
	"github.com/dobyte/due/session"
)

var (
	ErrInvalidGID         = errors.New("invalid gate id")
	ErrInvalidNID         = errors.New("invalid node id")
	ErrInvalidSessionKind = errors.New("invalid session kind")
	ErrNotFoundUserSource = errors.New("not found user source")
	ErrReceiveTargetEmpty = errors.New("the receive target is empty")
)

type Proxy interface {
	// AddRouteHandler 添加路由处理器
	AddRouteHandler(route int32, stateful bool, handler RouteHandler)
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
}

type proxy struct {
	node       *Node    // 节点
	sourceGate sync.Map // 用户来源网关
	sourceNode sync.Map // 用户来源节点
}

func newProxy(node *Node) *proxy {
	return &proxy{node: node}
}

// AddRouteHandler 添加路由处理器
func (p *proxy) AddRouteHandler(route int32, stateful bool, handler RouteHandler) {
	p.node.addRouteHandler(route, stateful, handler)
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

	if _, err = client.Bind(ctx, &pb.BindRequest{
		CID: cid,
		UID: uid,
	}); err != nil {
		return err
	}

	p.sourceGate.Store(uid, gid)

	return nil
}

// UnbindGate 解绑网关
func (p *proxy) UnbindGate(ctx context.Context, uid int64) error {
	_, err := p.doGateRPC(ctx, uid, func(client pb.GateClient) (bool, interface{}, error) {
		reply, err := client.Unbind(ctx, &pb.UnbindRequest{
			UID: uid,
		})

		return status.Code(err) == code.NotFoundSession, reply, err
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
	services, err := p.node.opts.registry.Services(ctx, kind.String())
	if err != nil {
		return nil, err
	}

	if len(states) == 0 {
		return services, nil
	}

	mp := make(map[cluster.State]bool, len(states))
	for _, state := range states {
		mp[state] = true
	}

	list := make([]*registry.ServiceInstance, 0, len(services))
	for i := range services {
		if mp[services[i].State] {
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

	reply, err := client.GetIP(ctx, &pb.GetIPRequest{
		NID:    p.node.opts.id,
		Kind:   int32(kind),
		Target: target,
	})
	if err != nil {
		return "", err
	}

	return reply.IP, nil
}

// 间接获取IP
func (p *proxy) indirectGetIP(ctx context.Context, uid int64) (string, error) {
	v, err := p.doGateRPC(ctx, uid, func(client pb.GateClient) (bool, interface{}, error) {
		reply, err := client.GetIP(ctx, &pb.GetIPRequest{
			NID:    p.node.opts.id,
			Kind:   int32(session.User),
			Target: uid,
		})

		return status.Code(err) == code.NotFoundSession, reply, err
	})
	if err != nil {
		return "", err
	}

	return v.(*pb.GetIPReply).IP, nil
}

// Push 推送消息
func (p *proxy) Push(ctx context.Context, args *PushArgs) error {
	switch args.Kind {
	case session.Conn:
		return p.directPush(ctx, args.GID, args.Kind, args.Target, args.Route, args.Message)
	case session.User:
		if args.GID == "" {
			return p.indirectPush(ctx, args.Target, args.Route, args.Message)
		} else {
			return p.directPush(ctx, args.GID, args.Kind, args.Target, args.Route, args.Message)
		}
	default:
		return ErrInvalidSessionKind
	}
}

// 直接推送
func (p *proxy) directPush(ctx context.Context, gid string, kind session.Kind, target int64, route int32, message interface{}) error {
	buffer, err := p.toBuffer(message)
	if err != nil {
		return err
	}

	client, err := p.getGateClientByGID(gid)
	if err != nil {
		return err
	}

	_, err = client.Push(ctx, &pb.PushRequest{
		NID:    p.node.opts.id,
		Kind:   int32(kind),
		Target: target,
		Route:  route,
		Buffer: buffer,
	})

	return err
}

// 间接推送
func (p *proxy) indirectPush(ctx context.Context, uid int64, route int32, message interface{}) error {
	buffer, err := p.toBuffer(message)
	if err != nil {
		return err
	}

	_, err = p.doGateRPC(ctx, uid, func(client pb.GateClient) (bool, interface{}, error) {
		reply, err := client.Push(ctx, &pb.PushRequest{
			NID:    p.node.opts.id,
			Kind:   int32(session.User),
			Target: uid,
			Route:  route,
			Buffer: buffer,
		})

		return status.Code(err) == code.NotFoundSession, reply, err
	})

	return err
}

// Response 响应消息
func (p *proxy) Response(ctx context.Context, req Request, message interface{}) error {
	return p.directPush(ctx, req.GID(), session.Conn, req.CID(), req.Route(), message)
}

// Multicast 推送组播消息
func (p *proxy) Multicast(ctx context.Context, args *MulticastArgs) (int64, error) {
	switch args.Kind {
	case session.Conn:
		return p.directMulticast(ctx, args.GID, args.Kind, args.Targets, args.Route, args.Message)
	case session.User:
		if args.GID == "" {
			return p.indirectMulticast(ctx, args.Targets, args.Route, args.Message)
		} else {
			return p.directMulticast(ctx, args.GID, args.Kind, args.Targets, args.Route, args.Message)
		}
	default:
		return 0, ErrInvalidSessionKind
	}
}

// 直接推送组播消息，只能推送到同一个网关服务器上
func (p *proxy) directMulticast(ctx context.Context, gid string, kind session.Kind, targets []int64, route int32, message interface{}) (int64, error) {
	if len(targets) == 0 {
		return 0, ErrReceiveTargetEmpty
	}

	buffer, err := p.toBuffer(message)
	if err != nil {
		return 0, err
	}

	client, err := p.getGateClientByGID(gid)
	if err != nil {
		return 0, err
	}

	reply, err := client.Multicast(ctx, &pb.MulticastRequest{
		NID:     p.node.opts.id,
		Kind:    int32(kind),
		Targets: targets,
		Route:   route,
		Buffer:  buffer,
	})
	if err != nil {
		return 0, err
	}

	return reply.Total, nil
}

// 间接推送组播消息
func (p *proxy) indirectMulticast(ctx context.Context, uids []int64, route int32, message interface{}) (int64, error) {
	total := int64(0)
	eg, ctx := errgroup.WithContext(ctx)

	for _, target := range uids {
		uid := target
		eg.Go(func() error {
			if err := p.indirectPush(ctx, uid, route, message); err != nil {
				return err
			}
			atomic.AddInt64(&total, 1)

			return nil
		})
	}

	err := eg.Wait()

	if total > 0 {
		return total, nil
	}
	return 0, err
}

// Broadcast 推送广播消息
func (p *proxy) Broadcast(ctx context.Context, args *BroadcastArgs) (int64, error) {
	buffer, err := p.toBuffer(args.Message)
	if err != nil {
		return 0, err
	}

	total := int64(0)
	eg, ctx := errgroup.WithContext(ctx)
	p.node.router.RangeGateEndpoint(func(insID string, ep *router.Endpoint) bool {
		eg.Go(func() error {
			client, err := p.newGateClient(ep)
			if err != nil {
				return err
			}

			reply, err := client.Broadcast(ctx, &pb.BroadcastRequest{
				NID:    p.node.opts.id,
				Kind:   int32(args.Kind),
				Route:  args.Route,
				Buffer: buffer,
			})
			if err != nil {
				return err
			}

			atomic.AddInt64(&total, reply.Total)

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
		return p.directDisconnect(ctx, args.GID, args.Kind, args.Target)
	case session.User:
		if args.GID == "" {
			return p.indirectDisconnect(ctx, args.Target)
		} else {
			return p.directDisconnect(ctx, args.GID, args.Kind, args.Target)
		}
	default:
		return ErrInvalidSessionKind
	}
}

// 直接断开连接
func (p *proxy) directDisconnect(ctx context.Context, gid string, kind session.Kind, target int64) error {
	client, err := p.getGateClientByGID(gid)
	if err != nil {
		return err
	}

	_, err = client.Disconnect(ctx, &pb.DisconnectRequest{
		NID:    p.node.opts.id,
		Kind:   int32(kind),
		Target: target,
	})

	return err
}

// 间接断开连接
func (p *proxy) indirectDisconnect(ctx context.Context, uid int64) error {
	_, err := p.doGateRPC(ctx, uid, func(client pb.GateClient) (bool, interface{}, error) {
		reply, err := client.Disconnect(ctx, &pb.DisconnectRequest{
			NID:    p.node.opts.id,
			Kind:   int32(session.User),
			Target: uid,
		})

		return status.Code(err) == code.NotFoundSession, reply, err
	})

	return err
}

//// Deliver 投递消息给当前节点处理
//func (p *proxy) Deliver(ctx context.Context, args *DeliverArgs) error {
//	var (
//		err       error
//		insID     string
//		lastInsID string
//		client    pb.GateClient
//		continued bool
//		reply     interface{}
//	)
//
//	for i := 0; i < 2; i++ {
//		if insID, err = p.LocateGate(ctx, args.UID); err != nil {
//			return err
//		}
//
//		if insID == lastInsID {
//			return reply, err
//		}
//
//		lastInsID = insID
//
//		if client, err = p.getGateClientByGID(insID); err != nil {
//			return nil, err
//		}
//
//		if continued, reply, err = fn(client); continued {
//			p.sourceGate.Delete(uid)
//			continue
//		}
//
//		break
//	}
//
//	return reply, err
//
//	p.node.chRead <- wrap{route: args.Route, request: &request{
//		gid:   args.GID,
//		nid:   args.NID,
//		cid:   args.CID,
//		uid:   args.UID,
//		msg:   args.Message,
//		codec: p.node.opts.codec,
//	}}
//}

// 消息转buffer
func (p *proxy) toBuffer(message interface{}) ([]byte, error) {
	if v, ok := message.([]byte); ok {
		return v, nil
	}

	return p.node.opts.codec.Marshal(message)
}

// 执行RPC调用
func (p *proxy) doGateRPC(ctx context.Context, uid int64, fn func(client pb.GateClient) (bool, interface{}, error)) (interface{}, error) {
	var (
		err       error
		gid       string
		lastGID   string
		client    pb.GateClient
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

		if client, err = p.getGateClientByGID(gid); err != nil {
			return nil, err
		}

		if continued, reply, err = fn(client); continued {
			p.sourceGate.Delete(uid)
			continue
		}

		break
	}

	return reply, err
}

// 执行RPC调用
func (p *proxy) doNodeRPC(ctx context.Context, route int32, uid int64, fn func(client pb.NodeClient) (bool, interface{}, error)) (interface{}, error) {
	var (
		err       error
		nid       string
		lastNID   string
		client    pb.NodeClient
		continued bool
		reply     interface{}
	)

	for i := 0; i < 2; i++ {
		if nid, err = p.LocateNode(ctx, uid); err != nil {
			return nil, err
		}

		if nid == lastNID {
			return reply, err
		}

		lastNID = nid

		if client, err = p.getNodeClientByNID(route, nid); err != nil {
			return nil, err
		}

		if continued, reply, err = fn(client); continued {
			p.sourceNode.Delete(uid)
			continue
		}

		break
	}

	return reply, err
}

// 根据实例ID获取网关客户端
func (p *proxy) getGateClientByGID(gid string) (pb.GateClient, error) {
	if gid == "" {
		return nil, ErrInvalidGID
	}

	ep, err := p.node.router.FindGateEndpoint(gid)
	if err != nil {
		return nil, err
	}

	return p.newGateClient(ep)
}

// 新建节点RPC客户端
func (p *proxy) newGateClient(ep *router.Endpoint) (pb.GateClient, error) {
	conn, err := grpc.Dial(ep.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return pb.NewGateClient(conn), nil
}

// 根据实例ID获取节点客户端
func (p *proxy) getNodeClientByNID(route int32, nid string) (pb.NodeClient, error) {
	if nid == "" {
		return nil, ErrInvalidNID
	}

	entity, err := p.node.router.FindNodeRoute(route)
	if err != nil {
		return nil, err
	}

	ep, err := entity.FindEndpoint(nid)
	if err != nil {
		return nil, err
	}

	return p.newNodeClient(ep)
}

// 新建节点RPC客户端
func (p *proxy) newNodeClient(ep *router.Endpoint) (pb.NodeClient, error) {
	conn, err := grpc.Dial(ep.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return pb.NewNodeClient(conn), nil
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
				case locator.SetLocation:
					source.Store(event.UID, event.InsID)
				case locator.RemLocation:
					source.Delete(event.UID)
				}
			}
		}
	}()
}
