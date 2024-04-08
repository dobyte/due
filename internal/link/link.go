package link

import (
	"context"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/dispatcher"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport"
	"golang.org/x/sync/errgroup"
	"sync"
	"sync/atomic"
	"time"
)

type Link struct {
	opts           *Options
	gateDispatcher *dispatcher.Dispatcher      // 网关分发器
	nodeDispatcher *dispatcher.Dispatcher      // 节点分发器
	gateSource     sync.Map                    // 用户来源网关
	rw             sync.RWMutex                // 锁
	nodeSources    map[int64]map[string]string // 用户来源节点
	transponders   []*Transponder
}

type Options struct {
	GID             string                     // 网关ID
	NID             string                     // 节点ID
	Codec           encoding.Codec             // 编解码器
	Locator         locate.Locator             // 定位器
	Registry        registry.Registry          // 注册器
	Encryptor       crypto.Encryptor           // 加密器
	Transporter     transport.Transporter      // 传输器
	BalanceStrategy dispatcher.BalanceStrategy // 负载均衡策略
}

func NewLink(opts *Options) *Link {
	l := &Link{
		opts:           opts,
		gateDispatcher: dispatcher.NewDispatcher(opts.BalanceStrategy),
		nodeDispatcher: dispatcher.NewDispatcher(opts.BalanceStrategy),
		nodeSources:    make(map[int64]map[string]string),
		transponders:   make([]*Transponder, 100),
	}

	//for i := 0; i < len(l.transponders); i++ {
	//	l.transponders[i] = NewTransponder(l)
	//}

	return l
}

// BindGate 绑定网关
func (l *Link) BindGate(ctx context.Context, uid int64, gid string, cid int64) error {
	client, err := l.getGateClientByGID(gid)
	if err != nil {
		return err
	}

	_, err = client.Bind(ctx, cid, uid)
	if err != nil {
		return err
	}

	l.gateSource.Store(uid, gid)

	return nil
}

// UnbindGate 解绑网关
func (l *Link) UnbindGate(ctx context.Context, uid int64) error {
	_, err := l.doGateRPC(ctx, uid, func(client transport.GateClient) (bool, interface{}, error) {
		miss, err := client.Unbind(ctx, uid)
		return miss, nil, err
	})
	if err != nil {
		return err
	}

	l.gateSource.Delete(uid)

	return nil
}

// BindNode 绑定节点
// 单个用户可以绑定到多个节点服务器上，相同名称的节点服务器只能绑定一个，多次绑定会到相同名称的节点服务器会覆盖之前的绑定。
// 绑定操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (l *Link) BindNode(ctx context.Context, uid int64, name, nid string) error {
	if l.opts.Locator == nil {
		return errors.ErrNotFoundLocator
	}

	err := l.opts.Locator.BindNode(ctx, uid, name, nid)
	if err != nil {
		return err
	}

	l.saveNodeSource(uid, name, nid)

	return nil
}

// UnbindNode 解绑节点
// 解绑时会对对应名称的节点服务器进行解绑，解绑时会对解绑节点ID进行校验，不匹配则解绑失败。
// 解绑操作会通过发布订阅方式同步到网关服务器和其他相关节点服务器上。
func (l *Link) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	if l.opts.Locator == nil {
		return errors.ErrNotFoundLocator
	}

	err := l.opts.Locator.UnbindNode(ctx, uid, name, nid)
	if err != nil {
		return err
	}

	l.deleteNodeSource(uid, name, nid)

	return nil
}

// 保存用户节点来源
func (l *Link) saveNodeSource(uid int64, name, nid string) {
	l.rw.Lock()
	defer l.rw.Unlock()

	sources, ok := l.nodeSources[uid]
	if !ok {
		sources = make(map[string]string)
		l.nodeSources[uid] = sources
	}
	sources[name] = nid
}

// 删除用户节点来源
func (l *Link) deleteNodeSource(uid int64, name, nid string) {
	l.rw.Lock()
	defer l.rw.Unlock()

	sources, ok := l.nodeSources[uid]
	if !ok {
		return
	}

	oldNID, ok := sources[name]
	if !ok {
		return
	}

	// ignore mismatched NID
	if oldNID != nid {
		return
	}

	if len(sources) == 1 {
		delete(l.nodeSources, uid)
	} else {
		delete(sources, name)
	}
}

// 加载用户节点来源
func (l *Link) loadNodeSource(uid int64, name string) (string, bool) {
	l.rw.RLock()
	defer l.rw.RUnlock()

	if source, ok := l.nodeSources[uid]; ok {
		if nid, ok := source[name]; ok {
			return nid, true
		}
	}

	return "", false
}

// LocateGate 定位用户所在网关
func (l *Link) LocateGate(ctx context.Context, uid int64) (string, error) {
	if l.opts.Locator == nil {
		return "", errors.ErrNotFoundLocator
	}

	if val, ok := l.gateSource.Load(uid); ok {
		if gid := val.(string); gid != "" {
			return gid, nil
		}
	}

	gid, err := l.opts.Locator.LocateGate(ctx, uid)
	if err != nil {
		return "", err
	}

	if gid == "" {
		return "", errors.ErrNotFoundUserLocation
	}

	l.gateSource.Store(uid, gid)

	return gid, nil
}

// AskGate 检测用户是否在给定的网关上
func (l *Link) AskGate(ctx context.Context, uid int64, gid string) (string, bool, error) {
	if l.opts.Locator == nil {
		return "", false, errors.ErrNotFoundLocator
	}

	if val, ok := l.gateSource.Load(uid); ok {
		return val.(string), val.(string) == gid, nil
	}

	insID, err := l.opts.Locator.LocateGate(ctx, uid)
	if err != nil {
		return "", false, err
	}

	if insID == "" {
		return "", false, errors.ErrNotFoundUserLocation
	}

	l.gateSource.Store(uid, insID)

	return insID, insID == gid, nil
}

// LocateNode 定位用户所在节点
func (l *Link) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	if l.opts.Locator == nil {
		return "", errors.ErrNotFoundLocator
	}

	nid, ok := l.loadNodeSource(uid, name)
	if ok {
		return nid, nil
	}

	nid, err := l.opts.Locator.LocateNode(ctx, uid, name)
	if err != nil {
		return "", err
	}

	if nid == "" {
		return "", errors.ErrNotFoundUserLocation
	}

	l.saveNodeSource(uid, name, nid)

	return nid, nil
}

// AskNode 检测用户是否在给定的节点上
func (l *Link) AskNode(ctx context.Context, uid int64, name, nid string) (string, bool, error) {
	if l.opts.Locator == nil {
		return "", false, errors.ErrNotFoundLocator
	}

	if insID, ok := l.loadNodeSource(uid, name); ok {
		return insID, insID == nid, nil
	}

	insID, err := l.opts.Locator.LocateNode(ctx, uid, name)
	if err != nil {
		return "", false, err
	}

	if insID == "" {
		return "", false, errors.ErrNotFoundUserLocation
	}

	l.saveNodeSource(uid, name, insID)

	return insID, insID == nid, nil
}

// FetchServiceList 拉取服务列表
func (l *Link) FetchServiceList(ctx context.Context, kind string, states ...string) ([]*registry.ServiceInstance, error) {
	services, err := l.opts.Registry.Services(ctx, kind)
	if err != nil {
		return nil, err
	}

	if len(states) == 0 {
		return services, nil
	}

	mp := make(map[string]struct{}, len(states))
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
func (l *Link) GetIP(ctx context.Context, args *GetIPArgs) (string, error) {
	switch args.Kind {
	case session.Conn:
		return l.directGetIP(ctx, args.GID, args.Kind, args.Target)
	case session.User:
		if args.GID == "" {
			return l.indirectGetIP(ctx, args.Target)
		} else {
			return l.directGetIP(ctx, args.GID, args.Kind, args.Target)
		}
	default:
		return "", errors.ErrInvalidSessionKind
	}
}

// 直接获取IP
func (l *Link) directGetIP(ctx context.Context, gid string, kind session.Kind, target int64) (string, error) {
	client, err := l.getGateClientByGID(gid)
	if err != nil {
		return "", err
	}

	ip, _, err := client.GetIP(ctx, kind, target)
	return ip, err
}

// 间接获取IP
func (l *Link) indirectGetIP(ctx context.Context, uid int64) (string, error) {
	v, err := l.doGateRPC(ctx, uid, func(client transport.GateClient) (bool, interface{}, error) {
		ip, miss, err := client.GetIP(ctx, session.User, uid)
		return miss, ip, err
	})
	if err != nil {
		return "", err
	}

	return v.(string), nil
}

// Push 推送消息
func (l *Link) Push(ctx context.Context, args *PushArgs) error {
	switch args.Kind {
	case session.Conn:
		return l.directPush(ctx, args)
	case session.User:
		if args.GID == "" {
			return l.indirectPush(ctx, args)
		} else {
			return l.directPush(ctx, args)
		}
	default:
		return errors.ErrInvalidSessionKind
	}
}

// 直接推送
func (l *Link) directPush(ctx context.Context, args *PushArgs) error {
	buffer, err := l.toBuffer(args.Message.Data, true)
	if err != nil {
		return err
	}

	client, err := l.getGateClientByGID(args.GID)
	if err != nil {
		return err
	}

	_, err = client.Push(ctx, args.Kind, args.Target, &packet.Message{
		Seq:    args.Message.Seq,
		Route:  args.Message.Route,
		Buffer: buffer,
	})
	return err
}

// 间接推送
func (l *Link) indirectPush(ctx context.Context, args *PushArgs) error {
	buffer, err := l.toBuffer(args.Message.Data, true)
	if err != nil {
		return err
	}

	_, err = l.doGateRPC(ctx, args.Target, func(client transport.GateClient) (bool, interface{}, error) {
		miss, err := client.Push(ctx, session.User, args.Target, &packet.Message{
			Seq:    args.Message.Seq,
			Route:  args.Message.Route,
			Buffer: buffer,
		})
		return miss, nil, err
	})

	return err
}

// Multicast 推送组播消息
func (l *Link) Multicast(ctx context.Context, args *MulticastArgs) (int64, error) {
	switch args.Kind {
	case session.Conn:
		return l.directMulticast(ctx, args)
	case session.User:
		if args.GID == "" {
			return l.indirectMulticast(ctx, args)
		} else {
			return l.directMulticast(ctx, args)
		}
	default:
		return 0, errors.ErrInvalidSessionKind
	}
}

// 直接推送组播消息，只能推送到同一个网关服务器上
func (l *Link) directMulticast(ctx context.Context, args *MulticastArgs) (int64, error) {
	if len(args.Targets) == 0 {
		return 0, errors.ErrReceiveTargetEmpty
	}

	buffer, err := l.toBuffer(args.Message.Data, true)
	if err != nil {
		return 0, err
	}

	client, err := l.getGateClientByGID(args.GID)
	if err != nil {
		return 0, err
	}

	return client.Multicast(ctx, args.Kind, args.Targets, &packet.Message{
		Seq:    args.Message.Seq,
		Route:  args.Message.Route,
		Buffer: buffer,
	})
}

// 间接推送组播消息
func (l *Link) indirectMulticast(ctx context.Context, args *MulticastArgs) (int64, error) {
	buffer, err := l.toBuffer(args.Message.Data, true)
	if err != nil {
		return 0, err
	}

	message := &packet.Message{
		Seq:    args.Message.Seq,
		Route:  args.Message.Route,
		Buffer: buffer,
	}

	total := int64(0)
	eg, ctx := errgroup.WithContext(ctx)
	for _, target := range args.Targets {
		func(target int64) {
			eg.Go(func() error {
				_, err := l.doGateRPC(ctx, target, func(client transport.GateClient) (bool, interface{}, error) {
					miss, err := client.Push(ctx, session.User, target, message)
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
func (l *Link) Broadcast(ctx context.Context, args *BroadcastArgs) (int64, error) {
	buffer, err := l.toBuffer(args.Message.Data, true)
	if err != nil {
		return 0, err
	}

	message := &packet.Message{
		Seq:    args.Message.Seq,
		Route:  args.Message.Route,
		Buffer: buffer,
	}

	total := int64(0)
	eg, ctx := errgroup.WithContext(ctx)
	l.gateDispatcher.IterateEndpoint(func(_ string, ep *endpoint.Endpoint) bool {
		eg.Go(func() error {
			client, err := l.opts.Transporter.NewGateClient(ep)
			if err != nil {
				return err
			}

			n, err := client.Broadcast(ctx, args.Kind, message)
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

// Stat 统计会话总数
func (l *Link) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	total := int64(0)
	eg, ctx := errgroup.WithContext(ctx)
	l.gateDispatcher.IterateEndpoint(func(_ string, ep *endpoint.Endpoint) bool {
		eg.Go(func() error {
			client, err := l.opts.Transporter.NewGateClient(ep)
			if err != nil {
				return err
			}

			n, err := client.Stat(ctx, kind)
			if err != nil {
				return err
			}

			atomic.AddInt64(&total, n)

			return nil
		})

		return true
	})

	err := eg.Wait()

	if total > 0 {
		return total, nil
	}

	return total, err
}

// Disconnect 断开连接
func (l *Link) Disconnect(ctx context.Context, args *DisconnectArgs) error {
	switch args.Kind {
	case session.Conn:
		return l.directDisconnect(ctx, args.GID, args.Kind, args.Target, args.IsForce)
	case session.User:
		if args.GID == "" {
			return l.indirectDisconnect(ctx, args.Target, args.IsForce)
		} else {
			return l.directDisconnect(ctx, args.GID, args.Kind, args.Target, args.IsForce)
		}
	default:
		return errors.ErrInvalidSessionKind
	}
}

// 直接断开连接
func (l *Link) directDisconnect(ctx context.Context, gid string, kind session.Kind, target int64, isForce bool) error {
	client, err := l.getGateClientByGID(gid)
	if err != nil {
		return err
	}

	_, err = client.Disconnect(ctx, kind, target, isForce)
	return err
}

// 间接断开连接
func (l *Link) indirectDisconnect(ctx context.Context, uid int64, isForce bool) error {
	_, err := l.doGateRPC(ctx, uid, func(client transport.GateClient) (bool, interface{}, error) {
		miss, err := client.Disconnect(ctx, session.User, uid, isForce)
		return miss, nil, err
	})

	return err
}

// Deliver 投递消息给节点处理
func (l *Link) Deliver(ctx context.Context, args *DeliverArgs) error {
	arguments := &transport.DeliverArgs{
		GID: l.opts.GID,
		NID: l.opts.NID,
		CID: args.CID,
		UID: args.UID,
	}

	switch msg := args.Message.(type) {
	case *packet.Message:
		arguments.Message = &packet.Message{
			Seq:    msg.Seq,
			Route:  msg.Route,
			Buffer: msg.Buffer,
		}
	case *Message:
		buffer, err := l.toBuffer(msg.Data, false)
		if err != nil {
			return err
		}
		arguments.Message = &packet.Message{
			Seq:    msg.Seq,
			Route:  msg.Route,
			Buffer: buffer,
		}
	default:
		return errors.ErrInvalidMessage
	}

	if args.NID != "" {
		client, err := l.getNodeClientByNID(args.NID)
		if err != nil {
			return err
		}

		_, err = client.Deliver(ctx, arguments)
		return err
	} else {
		_, err := l.doNodeRPC(ctx, arguments.Message.Route, args.UID, func(ctx context.Context, client transport.NodeClient) (bool, interface{}, error) {
			miss, err := client.Deliver(ctx, arguments)
			return miss, nil, err
		})
		if err != nil && !errors.Is(err, errors.ErrNotFoundUserLocation) {
			return err
		}
		return nil

		//index := int(arguments.CID % int64(len(l.transponders)))
		//l.transponders[index].deliver(arguments)
		//return nil
	}
}

// Trigger 触发事件
func (l *Link) Trigger(ctx context.Context, args *TriggerArgs) error {
	event, err := l.nodeDispatcher.FindEvent(args.Event)
	if err != nil {
		return err
	}

	arguments := &transport.TriggerArgs{
		Event: args.Event,
		GID:   l.opts.GID,
		CID:   args.CID,
		UID:   args.UID,
	}

	eg, ctx := errgroup.WithContext(ctx)
	event.IterateEndpoint(func(_ string, ep *endpoint.Endpoint) bool {
		eg.Go(func() error {
			client, err := l.opts.Transporter.NewNodeClient(ep)
			if err != nil {
				return err
			}

			_, err = client.Trigger(ctx, arguments)
			return err
		})

		return true
	})

	return eg.Wait()
}

// 执行网关RPC调用
func (l *Link) doGateRPC(ctx context.Context, uid int64, fn func(client transport.GateClient) (bool, interface{}, error)) (interface{}, error) {
	var (
		err       error
		gid       string
		prev      string
		client    transport.GateClient
		continued bool
		reply     interface{}
	)

	for i := 0; i < 2; i++ {
		if gid, err = l.LocateGate(ctx, uid); err != nil {
			return nil, err
		}

		if gid == prev {
			return reply, err
		}

		prev = gid

		client, err = l.getGateClientByGID(gid)
		if err != nil {
			return nil, err
		}

		continued, reply, err = fn(client)
		if continued {
			l.gateSource.Delete(uid)
			continue
		}

		break
	}

	return reply, err
}

// 执行节点RPC调用
func (l *Link) doNodeRPC(ctx context.Context, routeID int32, uid int64, fn func(ctx context.Context, client transport.NodeClient) (bool, interface{}, error)) (interface{}, error) {
	var (
		err       error
		nid       string
		prev      string
		client    transport.NodeClient
		route     *dispatcher.Route
		ep        *endpoint.Endpoint
		continued bool
		reply     interface{}
	)

	if route, err = l.nodeDispatcher.FindRoute(routeID); err != nil {
		return nil, err
	}

	if l.opts.GID != "" && route.Internal() {
		return nil, errors.ErrIllegalRequest
	}

	for i := 0; i < 2; i++ {
		if route.Stateful() {
			if nid, err = l.LocateNode(ctx, uid, route.Group()); err != nil {
				return nil, err
			}
			if nid == prev {
				return reply, err
			}
			prev = nid
		}

		ep, err = route.FindEndpoint(nid)
		if err != nil {
			return nil, err
		}

		client, err = l.opts.Transporter.NewNodeClient(ep)
		if err != nil {
			return nil, err
		}

		continued, reply, err = fn(ctx, client)
		if continued {
			if route.Stateful() {
				l.deleteNodeSource(uid, route.Group(), prev)
			}
			continue
		}

		break
	}

	return reply, err
}

// 消息转buffer
func (l *Link) toBuffer(message interface{}, encrypt bool) ([]byte, error) {
	if message == nil {
		return nil, nil
	}

	if v, ok := message.([]byte); ok {
		return v, nil
	}

	data, err := l.opts.Codec.Marshal(message)
	if err != nil {
		return nil, err
	}

	if encrypt && l.opts.Encryptor != nil {
		return l.opts.Encryptor.Encrypt(data)
	}

	return data, nil
}

// 根据实例ID获取网关客户端
func (l *Link) getGateClientByGID(gid string) (transport.GateClient, error) {
	if gid == "" {
		return nil, errors.ErrInvalidGID
	}

	ep, err := l.gateDispatcher.FindEndpoint(gid)
	if err != nil {
		return nil, err
	}

	return l.opts.Transporter.NewGateClient(ep)
}

// 根据实例ID获取节点客户端
func (l *Link) getNodeClientByNID(nid string) (transport.NodeClient, error) {
	if nid == "" {
		return nil, errors.ErrInvalidNID
	}

	ep, err := l.nodeDispatcher.FindEndpoint(nid)
	if err != nil {
		return nil, err
	}

	return l.opts.Transporter.NewNodeClient(ep)
}

// WatchServiceInstance 监听服务实例
func (l *Link) WatchServiceInstance(ctx context.Context, kinds ...string) {
	for _, kind := range kinds {
		l.watchServiceInstance(ctx, kind)
	}
}

// 监听服务实例
func (l *Link) watchServiceInstance(ctx context.Context, kind string) {
	rctx, rcancel := context.WithTimeout(ctx, 10*time.Second)
	watcher, err := l.opts.Registry.Watch(rctx, kind)
	rcancel()
	if err != nil {
		log.Fatalf("the dispatcher instance watch failed: %v", err)
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
			services, err := watcher.Next()
			if err != nil {
				continue
			}

			if kind == "node" {
				l.nodeDispatcher.ReplaceServices(services...)
			} else {
				l.gateDispatcher.ReplaceServices(services...)
			}
		}
	}()
}

// WatchUserLocate 监听用户定位
func (l *Link) WatchUserLocate(ctx context.Context, kinds ...string) {
	if l.opts.Locator == nil {
		return
	}

	rctx, rcancel := context.WithTimeout(ctx, 10*time.Second)
	watcher, err := l.opts.Locator.Watch(rctx, kinds...)
	rcancel()
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
				switch event.Type {
				case locate.BindGate:
					l.gateSource.Store(event.UID, event.InsID)
				case locate.BindNode:
					l.saveNodeSource(event.UID, event.InsName, event.InsID)
				case locate.UnbindGate:
					l.gateSource.Delete(event.UID)
				case locate.UnbindNode:
					l.deleteNodeSource(event.UID, event.InsName, event.InsID)
				}
			}
		}
	}()
}
