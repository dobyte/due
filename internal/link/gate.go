package link

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/dispatcher"
	"github.com/dobyte/due/v2/internal/transporter/gate"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/session"
	"golang.org/x/sync/errgroup"
)

type GateLinker struct {
	ctx        context.Context        // 上下文
	opts       *Options               // 参数项
	sources    sync.Map               // 用户源
	builder    *gate.Builder          // 构建器
	dispatcher *dispatcher.Dispatcher // 分发器
}

func NewGateLinker(ctx context.Context, opts *Options) *GateLinker {
	l := &GateLinker{
		ctx:        ctx,
		opts:       opts,
		builder:    gate.NewBuilder(&gate.Options{InsID: opts.InsID, InsKind: opts.InsKind}),
		dispatcher: dispatcher.NewDispatcher(opts.BalanceStrategy),
	}

	return l
}

// Ask 检测用户是否在给定的网关上
func (l *GateLinker) Ask(ctx context.Context, gid string, uid int64) (string, bool, error) {
	insID, err := l.Locate(ctx, uid)
	if err != nil {
		return "", false, err
	}

	return insID, insID == gid, nil
}

// Has 检测是否存在某个网关
func (l *GateLinker) Has(gid string) bool {
	_, err := l.dispatcher.FindEndpoint(gid)
	return err == nil
}

// Locate 定位用户所在网关
func (l *GateLinker) Locate(ctx context.Context, uid int64) (string, error) {
	if l.opts.Locator == nil {
		return "", errors.ErrNotFoundLocator
	}

	if val, ok := l.sources.Load(uid); ok {
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

	l.sources.Store(uid, gid)

	return gid, nil
}

// FetchGateList 拉取网关列表
func (l *GateLinker) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	services, err := l.opts.Registry.Services(ctx, cluster.Gate.String())
	if err != nil {
		return nil, err
	}

	if len(states) == 0 {
		return services, nil
	}

	mp := make(map[string]struct{}, len(states))
	for _, state := range states {
		mp[state.String()] = struct{}{}
	}

	list := make([]*registry.ServiceInstance, 0, len(services))
	for i := range services {
		if _, ok := mp[services[i].State]; ok {
			list = append(list, services[i])
		}
	}

	return list, nil
}

// Bind 绑定网关
func (l *GateLinker) Bind(ctx context.Context, gid string, cid, uid int64) error {
	client, err := l.doBuildClient(gid)
	if err != nil {
		return err
	}

	_, err = client.Bind(ctx, cid, uid)
	if err != nil {
		return err
	}

	l.sources.Store(uid, gid)

	return nil
}

// Unbind 解绑网关
func (l *GateLinker) Unbind(ctx context.Context, uid int64) error {
	_, err := l.doRPC(ctx, uid, func(client *gate.Client) (bool, interface{}, error) {
		miss, err := client.Unbind(ctx, uid)
		return miss, nil, err
	})
	if err != nil {
		return err
	}

	l.sources.Delete(uid)

	return nil
}

// GetState 获取网关状态
func (l *GateLinker) GetState(ctx context.Context, gid string) (cluster.State, error) {
	client, err := l.doBuildClient(gid)
	if err != nil {
		return cluster.Shut, err
	}

	return client.GetState(ctx)
}

// SetState 设置网关状态
func (l *GateLinker) SetState(ctx context.Context, gid string, state cluster.State) error {
	client, err := l.doBuildClient(gid)
	if err != nil {
		return err
	}

	return client.SetState(ctx, state)
}

// GetIP 获取客户端IP
func (l *GateLinker) GetIP(ctx context.Context, args *GetIPArgs) (string, error) {
	switch args.Kind {
	case session.Conn:
		return l.doDirectGetIP(ctx, args.GID, args.Kind, args.Target)
	case session.User:
		if args.GID == "" {
			return l.doIndirectGetIP(ctx, args.Target)
		} else {
			return l.doDirectGetIP(ctx, args.GID, args.Kind, args.Target)
		}
	default:
		return "", errors.ErrInvalidSessionKind
	}
}

// 直接获取IP
func (l *GateLinker) doDirectGetIP(ctx context.Context, gid string, kind session.Kind, target int64) (string, error) {
	client, err := l.doBuildClient(gid)
	if err != nil {
		return "", err
	}

	ip, _, err := client.GetIP(ctx, kind, target)
	return ip, err
}

// 间接获取IP
func (l *GateLinker) doIndirectGetIP(ctx context.Context, uid int64) (string, error) {
	v, err := l.doRPC(ctx, uid, func(client *gate.Client) (bool, interface{}, error) {
		ip, miss, err := client.GetIP(ctx, session.User, uid)
		return miss, ip, err
	})
	if err != nil {
		return "", err
	}

	return v.(string), nil
}

// Stat 统计会话总数
func (l *GateLinker) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	total := int64(0)
	eg, ctx := errgroup.WithContext(ctx)

	l.dispatcher.IterateEndpoint(func(_ string, ep *endpoint.Endpoint) bool {
		eg.Go(func() error {
			client, err := l.builder.Build(ep.Address())
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

// IsOnline 检测是否在线
func (l *GateLinker) IsOnline(ctx context.Context, args *IsOnlineArgs) (bool, error) {
	switch args.Kind {
	case session.Conn:
		return l.doDirectIsOnline(ctx, args)
	case session.User:
		if args.GID == "" {
			return l.doIndirectIsOnline(ctx, args)
		} else {
			return l.doDirectIsOnline(ctx, args)
		}
	default:
		return false, errors.ErrInvalidSessionKind
	}
}

// 直接检测是否在线
func (l *GateLinker) doDirectIsOnline(ctx context.Context, args *IsOnlineArgs) (bool, error) {
	client, err := l.doBuildClient(args.GID)
	if err != nil {
		return false, err
	}

	_, isOnline, err := client.IsOnline(ctx, args.Kind, args.Target)
	return isOnline, err
}

// 间接检测是否在线
func (l *GateLinker) doIndirectIsOnline(ctx context.Context, args *IsOnlineArgs) (bool, error) {
	v, err := l.doRPC(ctx, args.Target, func(client *gate.Client) (bool, interface{}, error) {
		return client.IsOnline(ctx, args.Kind, args.Target)
	})
	if err != nil {
		return false, err
	}

	return v.(bool), nil
}

// Disconnect 断开连接
func (l *GateLinker) Disconnect(ctx context.Context, args *DisconnectArgs) error {
	switch args.Kind {
	case session.Conn:
		return l.doDirectDisconnect(ctx, args)
	case session.User:
		if args.GID == "" {
			return l.doIndirectDisconnect(ctx, args.Target, args.Force)
		} else {
			return l.doDirectDisconnect(ctx, args)
		}
	default:
		return errors.ErrInvalidSessionKind
	}
}

// 直接断开连接
func (l *GateLinker) doDirectDisconnect(ctx context.Context, args *DisconnectArgs) error {
	client, err := l.doBuildClient(args.GID)
	if err != nil {
		return err
	}

	return client.Disconnect(ctx, args.Kind, args.Target, args.Force)
}

// 间接断开连接
func (l *GateLinker) doIndirectDisconnect(ctx context.Context, uid int64, force bool) error {
	_, err := l.doRPC(ctx, uid, func(client *gate.Client) (bool, interface{}, error) {
		return false, nil, client.Disconnect(ctx, session.User, uid, force)
	})

	return err
}

// Push 推送消息
func (l *GateLinker) Push(ctx context.Context, args *PushArgs) error {
	switch args.Kind {
	case session.Conn:
		return l.doDirectPush(ctx, args)
	case session.User:
		if args.GID == "" {
			return l.doIndirectPush(ctx, args)
		} else {
			return l.doDirectPush(ctx, args)
		}
	default:
		return errors.ErrInvalidSessionKind
	}
}

// 直接推送
func (l *GateLinker) doDirectPush(ctx context.Context, args *PushArgs) error {
	message, err := l.PackMessage(args.Message, true)
	if err != nil {
		return err
	}

	client, err := l.doBuildClient(args.GID)
	if err != nil {
		return err
	}

	return client.Push(ctx, args.Kind, args.Target, message)
}

// 间接推送
func (l *GateLinker) doIndirectPush(ctx context.Context, args *PushArgs) error {
	message, err := l.PackMessage(args.Message, true)
	if err != nil {
		return err
	}

	_, err = l.doRPC(ctx, args.Target, func(client *gate.Client) (bool, interface{}, error) {
		return false, nil, client.Push(ctx, args.Kind, args.Target, message)
	})

	return err
}

// Multicast 推送组播消息
func (l *GateLinker) Multicast(ctx context.Context, args *MulticastArgs) error {
	switch args.Kind {
	case session.Conn:
		return l.doDirectMulticast(ctx, args)
	case session.User:
		if args.GID == "" {
			return l.doIndirectMulticast(ctx, args)
		} else {
			return l.doDirectMulticast(ctx, args)
		}
	default:
		return errors.ErrInvalidSessionKind
	}
}

// 直接推送组播消息，只能推送到同一个网关服务器上
func (l *GateLinker) doDirectMulticast(ctx context.Context, args *MulticastArgs) error {
	if len(args.Targets) == 0 {
		return errors.ErrReceiveTargetEmpty
	}

	message, err := l.PackMessage(args.Message, true)
	if err != nil {
		return err
	}

	client, err := l.doBuildClient(args.GID)
	if err != nil {
		return err
	}

	return client.Multicast(ctx, args.Kind, args.Targets, message)
}

// 间接推送组播消息
func (l *GateLinker) doIndirectMulticast(ctx context.Context, args *MulticastArgs) error {
	if len(args.Targets) == 0 {
		return errors.ErrReceiveTargetEmpty
	}

	buf, err := l.PackBuffer(args.Message.Data, true)
	if err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)

	for _, target := range args.Targets {
		func(target int64) {
			eg.Go(func() error {
				message, err := packet.PackBuffer(&packet.Message{
					Seq:    args.Message.Seq,
					Route:  args.Message.Route,
					Buffer: buf,
				})
				if err != nil {
					return err
				}

				_, err = l.doRPC(ctx, target, func(client *gate.Client) (bool, interface{}, error) {
					return false, nil, client.Push(ctx, args.Kind, target, message)
				})
				return err
			})
		}(target)
	}

	return eg.Wait()
}

// Broadcast 推送广播消息
func (l *GateLinker) Broadcast(ctx context.Context, args *BroadcastArgs) error {
	buf, err := l.PackBuffer(args.Message.Data, true)
	if err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)

	l.dispatcher.IterateEndpoint(func(_ string, ep *endpoint.Endpoint) bool {
		eg.Go(func() error {
			message, err := packet.PackBuffer(&packet.Message{
				Seq:    args.Message.Seq,
				Route:  args.Message.Route,
				Buffer: buf,
			})
			if err != nil {
				return err
			}

			client, err := l.builder.Build(ep.Address())
			if err != nil {
				return err
			}

			return client.Broadcast(ctx, args.Kind, message)
		})

		return true
	})

	return eg.Wait()
}

// Publish 发布频道消息
func (l *GateLinker) Publish(ctx context.Context, args *PublishArgs) error {
	buf, err := l.PackBuffer(args.Message.Data, true)
	if err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)

	l.dispatcher.IterateEndpoint(func(_ string, ep *endpoint.Endpoint) bool {
		eg.Go(func() error {
			message, err := packet.PackBuffer(&packet.Message{
				Seq:    args.Message.Seq,
				Route:  args.Message.Route,
				Buffer: buf,
			})
			if err != nil {
				return err
			}

			client, err := l.builder.Build(ep.Address())
			if err != nil {
				return err
			}

			return client.Publish(ctx, args.Channel, message)
		})

		return true
	})

	return eg.Wait()
}

// Subscribe 订阅频道
func (l *GateLinker) Subscribe(ctx context.Context, args *SubscribeArgs) error {
	switch args.Kind {
	case session.Conn:
		return l.doDirectSubscribe(ctx, args)
	case session.User:
		if args.GID == "" {
			return l.doIndirectSubscribe(ctx, args)
		} else {
			return l.doDirectSubscribe(ctx, args)
		}
	default:
		return errors.ErrInvalidSessionKind
	}
}

// 直接订阅频道，只能订阅同一个网关服务器上
func (l *GateLinker) doDirectSubscribe(ctx context.Context, args *SubscribeArgs) error {
	if len(args.Targets) == 0 {
		return errors.ErrReceiveTargetEmpty
	}

	client, err := l.doBuildClient(args.GID)
	if err != nil {
		return err
	}

	return client.Subscribe(ctx, args.Kind, args.Targets, args.Channel)
}

// 间接订阅频道
func (l *GateLinker) doIndirectSubscribe(ctx context.Context, args *SubscribeArgs) error {
	if len(args.Targets) == 0 {
		return errors.ErrReceiveTargetEmpty
	}

	eg, ctx := errgroup.WithContext(ctx)

	for _, target := range args.Targets {
		func(target int64) {
			eg.Go(func() error {
				_, err := l.doRPC(ctx, target, func(client *gate.Client) (bool, interface{}, error) {
					return false, nil, client.Subscribe(ctx, args.Kind, []int64{target}, args.Channel)
				})
				return err
			})
		}(target)
	}

	return eg.Wait()
}

// Unsubscribe 取消订阅频道
func (l *GateLinker) Unsubscribe(ctx context.Context, args *UnsubscribeArgs) error {
	switch args.Kind {
	case session.Conn:
		return l.doDirectUnsubscribe(ctx, args)
	case session.User:
		if args.GID == "" {
			return l.doIndirectUnsubscribe(ctx, args)
		} else {
			return l.doDirectUnsubscribe(ctx, args)
		}
	default:
		return errors.ErrInvalidSessionKind
	}
}

// 直接订阅频道，只能订阅同一个网关服务器上
func (l *GateLinker) doDirectUnsubscribe(ctx context.Context, args *UnsubscribeArgs) error {
	if len(args.Targets) == 0 {
		return errors.ErrReceiveTargetEmpty
	}

	client, err := l.doBuildClient(args.GID)
	if err != nil {
		return err
	}

	return client.Unsubscribe(ctx, args.Kind, args.Targets, args.Channel)
}

// 间接订阅频道
func (l *GateLinker) doIndirectUnsubscribe(ctx context.Context, args *UnsubscribeArgs) error {
	if len(args.Targets) == 0 {
		return errors.ErrReceiveTargetEmpty
	}

	eg, ctx := errgroup.WithContext(ctx)

	for _, target := range args.Targets {
		func(target int64) {
			eg.Go(func() error {
				_, err := l.doRPC(ctx, target, func(client *gate.Client) (bool, interface{}, error) {
					return false, nil, client.Unsubscribe(ctx, args.Kind, []int64{target}, args.Channel)
				})
				return err
			})
		}(target)
	}

	return eg.Wait()
}

// 执行RPC调用
func (l *GateLinker) doRPC(ctx context.Context, uid int64, fn func(client *gate.Client) (bool, interface{}, error)) (interface{}, error) {
	var (
		err       error
		gid       string
		prev      string
		client    *gate.Client
		continued bool
		reply     interface{}
	)

	for i := 0; i < 2; i++ {
		if gid, err = l.Locate(ctx, uid); err != nil {
			return nil, err
		}

		if gid == prev {
			return reply, err
		}

		prev = gid

		client, err = l.doBuildClient(gid)
		if err != nil {
			return nil, err
		}

		continued, reply, err = fn(client)
		if continued {
			l.sources.Delete(uid)
			continue
		}

		break
	}

	return reply, err
}

// 构建网关客户端
func (l *GateLinker) doBuildClient(gid string) (*gate.Client, error) {
	if gid == "" {
		return nil, errors.ErrInvalidGID
	}

	ep, err := l.dispatcher.FindEndpoint(gid)
	if err != nil {
		return nil, err
	}

	return l.builder.Build(ep.Address())
}

// PackMessage 打包消息
func (l *GateLinker) PackMessage(message *Message, encrypt bool) (buffer.Buffer, error) {
	buf, err := l.PackBuffer(message.Data, encrypt)
	if err != nil {
		return nil, err
	}

	return packet.PackBuffer(&packet.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: buf,
	})
}

// PackBuffer 消息转buffer
func (l *GateLinker) PackBuffer(message interface{}, encrypt bool) ([]byte, error) {
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

// WatchUserLocate 监听用户定位
func (l *GateLinker) WatchUserLocate() {
	if l.opts.Locator == nil {
		return
	}

	ctx, cancel := context.WithTimeout(l.ctx, 3*time.Second)
	watcher, err := l.opts.Locator.Watch(ctx, cluster.Gate.String())
	cancel()
	if err != nil {
		log.Fatalf("user locate event watch failed: %v", err)
	}

	go func() {
		defer watcher.Stop()
		for {
			select {
			case <-l.ctx.Done():
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
					l.sources.Store(event.UID, event.InsID)
				case locate.UnbindGate:
					l.sources.Delete(event.UID)
				default:
					// ignore
				}
			}
		}
	}()
}

// WatchClusterInstance 监听集群实例
func (l *GateLinker) WatchClusterInstance() {
	ctx, cancel := context.WithTimeout(l.ctx, 3*time.Second)
	watcher, err := l.opts.Registry.Watch(ctx, cluster.Gate.String())
	cancel()
	if err != nil {
		log.Fatalf("the dispatcher instance watch failed: %v", err)
	}

	go func() {
		defer watcher.Stop()
		for {
			select {
			case <-l.ctx.Done():
				return
			default:
				// exec watch
			}

			services, err := watcher.Next()
			if err != nil {
				continue
			}

			l.dispatcher.ReplaceServices(services...)
		}
	}()
}
