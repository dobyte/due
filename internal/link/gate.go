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
		dispatcher: dispatcher.NewDispatcher(opts.Dispatch),
	}

	return l
}

// HasGate 检测是否存在某个网关
func (l *GateLinker) HasGate(gid string) bool {
	_, err := l.dispatcher.FindEndpoint(gid)
	return err == nil
}

// AskGate 检测用户是否在给定的网关上
func (l *GateLinker) AskGate(ctx context.Context, gid string, uid int64) (string, bool, error) {
	insID, err := l.LocateGate(ctx, uid)
	if err != nil {
		return "", false, err
	}

	return insID, insID == gid, nil
}

// LocateGate 定位用户所在网关
func (l *GateLinker) LocateGate(ctx context.Context, uid int64) (string, error) {
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

// BindGate 绑定网关
func (l *GateLinker) BindGate(ctx context.Context, gid string, cid, uid int64) error {
	client, err := l.doBuildClient(gid)
	if err != nil {
		return err
	}

	if err = client.Bind(ctx, cid, uid); err != nil {
		return err
	}

	l.sources.Store(uid, gid)

	return nil
}

// UnbindGate 解绑网关
func (l *GateLinker) UnbindGate(ctx context.Context, uid int64) error {
	if _, err := l.doRPC(ctx, uid, func(client *gate.Client, index, total int) (bool, any, error) {
		if err := client.Unbind(ctx, uid); err != nil {
			return errors.Is(err, errors.ErrNotFoundSession), nil, err
		} else {
			return false, nil, nil
		}
	}); err != nil {
		return err
	}

	l.sources.Delete(uid)

	return nil
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

	return client.GetIP(ctx, kind, target)
}

// 间接获取IP
func (l *GateLinker) doIndirectGetIP(ctx context.Context, uid int64) (string, error) {
	v, err := l.doRPC(ctx, uid, func(client *gate.Client, index, total int) (bool, any, error) {
		if ip, err := client.GetIP(ctx, session.User, uid); err != nil {
			return errors.Is(err, errors.ErrNotFoundSession), ip, err
		} else {
			return false, ip, nil
		}
	})
	if err != nil {
		return "", err
	}

	return v.(string), nil
}

// Stat 统计会话总数
func (l *GateLinker) Stat(ctx context.Context, kind session.Kind) (total int64, err error) {
	eg, ctx := errgroup.WithContext(ctx)

	l.dispatcher.VisitEndpoints(func(_ string, ep *endpoint.Endpoint) bool {
		eg.Go(func() error {
			client, err := l.builder.Build(ep.Address())
			if err != nil {
				return err
			}

			if n, err := client.Stat(ctx, kind); err != nil {
				return err
			} else {
				atomic.AddInt64(&total, n)

				return nil
			}
		})

		return true
	})

	err = eg.Wait()

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

	return client.IsOnline(ctx, args.Kind, args.Target)
}

// 间接检测是否在线
func (l *GateLinker) doIndirectIsOnline(ctx context.Context, args *IsOnlineArgs) (bool, error) {
	v, err := l.doRPC(ctx, args.Target, func(client *gate.Client, index, total int) (bool, any, error) {
		if isOnline, err := client.IsOnline(ctx, args.Kind, args.Target); err != nil {
			return errors.Is(err, errors.ErrNotFoundSession), isOnline, err
		} else {
			return false, isOnline, nil
		}
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
	_, err := l.doRPC(ctx, uid, func(client *gate.Client, index, total int) (bool, any, error) {
		if err := client.Disconnect(ctx, session.User, uid, force); err != nil {
			return errors.Is(err, errors.ErrNotFoundSession), nil, err
		} else {
			return false, nil, nil
		}
	})

	return err
}

// Push 推送消息
func (l *GateLinker) Push(ctx context.Context, args *PushArgs) error {
	_, err := l.Multicast(ctx, &cluster.MulticastArgs{
		GID:     args.GID,
		Kind:    args.Kind,
		Targets: []int64{args.Target},
		Message: args.Message,
		Ack:     args.Ack,
	})

	return err
}

// 执行推送消息
func (l *GateLinker) doPush(ctx context.Context, kind session.Kind, target int64, message buffer.Buffer, ack bool) error {
	_, err := l.doRPC(ctx, target, func(client *gate.Client, index, total int) (bool, any, error) {
		if err := client.Push(ctx, kind, target, message, ack); ack {
			if errors.Is(err, errors.ErrNotFoundSession) {
				return true, nil, err
			} else {
				for range total - index {
					message.Release()
				}

				return false, nil, err
			}
		} else {
			return false, nil, err
		}
	}, func(index, total int) {
		for range total - index {
			message.Release()
		}
	})

	return err
}

// Multicast 推送组播消息
// 要想获得推送成功的目标数，需将args.Ack设为true
func (l *GateLinker) Multicast(ctx context.Context, args *MulticastArgs) (int64, error) {
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
		return 0, errors.ErrInvalidSessionKind
	}
}

// 直接推送组播消息，只能推送到同一个网关服务器上
func (l *GateLinker) doDirectMulticast(ctx context.Context, args *MulticastArgs) (int64, error) {
	if len(args.Targets) == 0 {
		return 0, errors.ErrReceiveTargetEmpty
	}

	client, err := l.doBuildClient(args.GID)
	if err != nil {
		return 0, err
	}

	message, err := l.PackMessage(args.Message, true)
	if err != nil {
		return 0, err
	}

	return client.Multicast(ctx, args.Kind, args.Targets, message, args.Ack)
}

// 间接推送组播消息
func (l *GateLinker) doIndirectMulticast(ctx context.Context, args *MulticastArgs) (int64, error) {
	n := len(args.Targets)

	if n == 0 {
		return 0, errors.ErrReceiveTargetEmpty
	}

	message, err := l.PackMessage(args.Message, true)
	if err != nil {
		return 0, err
	}

	if args.Ack {
		message.Delay(int32(n * 2))

		if n == 1 {
			if err := l.doPush(ctx, args.Kind, args.Targets[0], message, args.Ack); err != nil {
				return 0, err
			} else {
				return 1, nil
			}
		} else {
			return l.doMulticast(ctx, args.Kind, args.Targets, message, args.Ack)
		}
	} else {
		if n == 1 {
			return 0, l.doPush(ctx, args.Kind, args.Targets[0], message, args.Ack)
		} else {
			message.Delay(int32(n))

			if _, err := l.doMulticast(ctx, args.Kind, args.Targets, message, args.Ack); err != nil {
				return 0, err
			} else {
				return 0, nil
			}
		}
	}
}

// 执行推送组播消息
func (l *GateLinker) doMulticast(ctx context.Context, kind session.Kind, targets []int64, message buffer.Buffer, ack bool) (total int64, err error) {
	eg, ctx := errgroup.WithContext(ctx)

	for i := range targets {
		target := targets[i]

		eg.Go(func() error {
			if err := l.doPush(ctx, kind, target, message, ack); err != nil {
				return err
			}

			atomic.AddInt64(&total, 1)

			return nil
		})
	}

	if err = eg.Wait(); err != nil && total == 0 {
		return 0, err
	} else {
		return total, nil
	}
}

// Broadcast 推送广播消息
func (l *GateLinker) Broadcast(ctx context.Context, args *BroadcastArgs) (int64, error) {
	var (
		endpoints = l.dispatcher.Endpoints()
		n         = len(endpoints)
	)

	if n == 0 {
		return 0, nil
	}

	message, err := l.PackMessage(args.Message, true)
	if err != nil {
		return 0, err
	}

	if n == 1 {
		for _, ep := range endpoints {
			return l.doBroadcast(ctx, ep.Address(), args.Kind, message, args.Ack)
		}

		return 0, nil
	} else {
		var (
			total   int64
			eg, ctx = errgroup.WithContext(ctx)
		)

		message.Delay(int32(n))

		for _, ep := range endpoints {
			addr := ep.Address()

			eg.Go(func() error {
				if v, err := l.doBroadcast(ctx, addr, args.Kind, message, args.Ack); err != nil {
					return err
				} else {
					atomic.AddInt64(&total, v)

					return nil
				}
			})
		}

		if err = eg.Wait(); err != nil && total == 0 {
			return 0, err
		} else {
			return total, nil
		}
	}
}

// 执行广播消息
func (l *GateLinker) doBroadcast(ctx context.Context, addr string, kind session.Kind, message buffer.Buffer, ack bool) (int64, error) {
	if client, err := l.builder.Build(addr); err != nil {
		message.Release()

		return 0, err
	} else {
		return client.Broadcast(ctx, kind, message, ack)
	}
}

// Publish 发布频道消息
func (l *GateLinker) Publish(ctx context.Context, args *PublishArgs) (int64, error) {
	var (
		endpoints = l.dispatcher.Endpoints()
		n         = len(endpoints)
	)

	if n == 0 {
		return 0, nil
	}

	message, err := l.PackMessage(args.Message, true)
	if err != nil {
		return 0, err
	}

	if n == 1 {
		for _, ep := range endpoints {
			return l.doPublish(ctx, ep.Address(), args.Channel, message, args.Ack)
		}

		return 0, nil
	} else {
		var (
			total   int64
			eg, ctx = errgroup.WithContext(ctx)
		)

		message.Delay(int32(n))

		for _, ep := range endpoints {
			addr := ep.Address()

			eg.Go(func() error {
				if v, err := l.doPublish(ctx, addr, args.Channel, message, args.Ack); err != nil {
					return err
				} else {
					atomic.AddInt64(&total, v)

					return nil
				}
			})
		}

		if err = eg.Wait(); err != nil && total == 0 {
			return 0, err
		} else {
			return total, nil
		}
	}
}

// 执行发布频道消息
func (l *GateLinker) doPublish(ctx context.Context, addr string, channel string, message buffer.Buffer, ack bool) (int64, error) {
	if client, err := l.builder.Build(addr); err != nil {
		message.Release()

		return 0, err
	} else {
		return client.Publish(ctx, channel, message, ack)
	}
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
				_, err := l.doRPC(ctx, target, func(client *gate.Client, index, total int) (bool, any, error) {
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
				_, err := l.doRPC(ctx, target, func(client *gate.Client, index, total int) (bool, any, error) {
					return false, nil, client.Unsubscribe(ctx, args.Kind, []int64{target}, args.Channel)
				})
				return err
			})
		}(target)
	}

	return eg.Wait()
}

// 执行RPC调用
func (l *GateLinker) doRPC(ctx context.Context, uid int64, successHandler func(client *gate.Client, index int, total int) (bool, any, error), failedHandler ...func(index int, total int)) (any, error) {
	var (
		err       error
		gid       string
		prev      string
		client    *gate.Client
		continued bool
		reply     any
		total     = 2
	)

	for i := range total {
		if gid, err = l.LocateGate(ctx, uid); err != nil {
			if len(failedHandler) > 0 {
				failedHandler[0](i+1, total)
			}
			return nil, err
		}

		if gid == prev {
			if len(failedHandler) > 0 {
				failedHandler[0](i+1, total)
			}
			return reply, err
		}

		prev = gid

		if client, err = l.doBuildClient(gid); err != nil {
			if len(failedHandler) > 0 {
				failedHandler[0](i+1, total)
			}
			return nil, err
		}

		if continued, reply, err = successHandler(client, i+1, total); !continued {
			break
		}

		l.sources.Delete(uid)
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
func (l *GateLinker) PackMessage(message *Message, encrypt bool) (*buffer.NocopyBuffer, error) {
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
func (l *GateLinker) PackBuffer(message any, encrypt bool) ([]byte, error) {
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
