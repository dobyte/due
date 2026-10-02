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
	ctx        context.Context        // Context
	opts       *Options               // Options
	sources    sync.Map               // User sources
	builder    *gate.Builder          // Builder
	dispatcher *dispatcher.Dispatcher // Dispatcher
}

func NewGateLinker(ctx context.Context, opts *Options) *GateLinker {
	l := &GateLinker{
		ctx:        ctx,
		opts:       opts,
		dispatcher: dispatcher.NewDispatcher(opts.Dispatch),
		builder: gate.NewBuilder(&gate.ClientOptions{
			ID:                opts.ID,
			Kind:              opts.Kind,
			ConnNum:           opts.ConnNum,
			CallTimeout:       opts.CallTimeout,
			DialTimeout:       opts.DialTimeout,
			DialRetryTimes:    opts.DialRetryTimes,
			WriteTimeout:      opts.CommandWriteTimeout,
			WriteQueueSize:    opts.CommandQueueSize,
			FaultRecoveryTime: opts.FaultRecoveryTime,
		}),
	}

	return l
}

// HasGate reports whether the gate identified by gid exists.
func (l *GateLinker) HasGate(gid string) bool {
	_, err := l.dispatcher.FindEndpoint(gid)
	return err == nil
}

// AskGate reports whether the user is located on the gate identified by gid.
func (l *GateLinker) AskGate(ctx context.Context, gid string, uid int64) (string, bool, error) {
	insID, err := l.LocateGate(ctx, uid)
	if err != nil {
		return "", false, err
	}

	return insID, insID == gid, nil
}

// LocateGate returns the gate where the user is located.
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

// BindGate binds the connection cid and user uid to the gate identified by gid.
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

// UnbindGate unbinds the user from its gate.
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

// FetchGateList returns the gate services, optionally filtered by states.
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

// GetState returns the state of the gate identified by gid.
func (l *GateLinker) GetState(ctx context.Context, gid string) (cluster.State, error) {
	client, err := l.doBuildClient(gid)
	if err != nil {
		return cluster.Shut, err
	}

	return client.GetState(ctx)
}

// SetState sets the state of the gate identified by gid.
func (l *GateLinker) SetState(ctx context.Context, gid string, state cluster.State) error {
	client, err := l.doBuildClient(gid)
	if err != nil {
		return err
	}

	return client.SetState(ctx, state)
}

// GetIP returns the client IP for the given session target.
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

// doDirectGetIP gets the IP directly from the gate identified by gid.
func (l *GateLinker) doDirectGetIP(ctx context.Context, gid string, kind session.Kind, target int64) (string, error) {
	client, err := l.doBuildClient(gid)
	if err != nil {
		return "", err
	}

	return client.GetIP(ctx, kind, target)
}

// doIndirectGetIP locates the user's gate and gets the IP from it.
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

// Stat returns the total number of sessions of the given kind across all gates.
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

// IsOnline reports whether the session target is online.
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

// doDirectIsOnline checks whether the target is online directly on its gate.
func (l *GateLinker) doDirectIsOnline(ctx context.Context, args *IsOnlineArgs) (bool, error) {
	client, err := l.doBuildClient(args.GID)
	if err != nil {
		return false, err
	}

	return client.IsOnline(ctx, args.Kind, args.Target)
}

// doIndirectIsOnline locates the user's gate and checks whether the user is online there.
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

// Disconnect disconnects the session target.
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

// doDirectDisconnect disconnects the session directly on its gate.
func (l *GateLinker) doDirectDisconnect(ctx context.Context, args *DisconnectArgs) error {
	client, err := l.doBuildClient(args.GID)
	if err != nil {
		return err
	}

	return client.Disconnect(ctx, args.Kind, args.Target, args.Force)
}

// doIndirectDisconnect locates the user's gate and disconnects the user there.
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

// Push pushes a message to a single target.
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

// doPush pushes a message to a single target through the gate that owns it.
func (l *GateLinker) doPush(ctx context.Context, kind session.Kind, target int64, disconnect bool, message buffer.Buffer, ack bool) error {
	_, err := l.doRPC(ctx, target, func(client *gate.Client, index, total int) (bool, any, error) {
		if err := client.Push(ctx, kind, target, disconnect, message, ack); ack {
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

// Multicast pushes a message to multiple targets.
//
// Set args.Ack to true to obtain the number of targets the message was pushed to successfully.
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

// doDirectMulticast pushes a message to multiple targets on the same gate server.
func (l *GateLinker) doDirectMulticast(ctx context.Context, args *MulticastArgs) (int64, error) {
	n := len(args.Targets)

	if n == 0 {
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

	if n == 1 {
		if err := client.Push(ctx, args.Kind, args.Targets[0], args.Disconnect, message, args.Ack); err != nil {
			return 0, err
		} else {
			if args.Ack {
				return 1, nil
			} else {
				return 0, nil
			}
		}
	} else {
		return client.Multicast(ctx, args.Kind, args.Targets, args.Disconnect, message, args.Ack)
	}
}

// doIndirectMulticast pushes a message to multiple targets by locating each target's gate.
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
		message.Delay(n * 2)

		if n == 1 {
			if err := l.doPush(ctx, args.Kind, args.Targets[0], args.Disconnect, message, args.Ack); err != nil {
				return 0, err
			} else {
				return 1, nil
			}
		} else {
			return l.doMulticast(ctx, args.Kind, args.Targets, args.Disconnect, message, args.Ack)
		}
	} else {
		if n == 1 {
			return 0, l.doPush(ctx, args.Kind, args.Targets[0], args.Disconnect, message, args.Ack)
		} else {
			message.Delay(n)

			if _, err := l.doMulticast(ctx, args.Kind, args.Targets, args.Disconnect, message, args.Ack); err != nil {
				return 0, err
			} else {
				return 0, nil
			}
		}
	}
}

// doMulticast pushes a message to each target concurrently and reports how many succeeded.
func (l *GateLinker) doMulticast(ctx context.Context, kind session.Kind, targets []int64, disconnect bool, message buffer.Buffer, ack bool) (total int64, err error) {
	eg, ctx := errgroup.WithContext(ctx)

	for i := range targets {
		target := targets[i]

		eg.Go(func() error {
			if err = l.doPush(ctx, kind, target, disconnect, message, ack); err != nil {
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

// Broadcast pushes a broadcast message to every gate.
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
			return l.doBroadcast(ctx, ep.Address(), args.Kind, args.Disconnect, message, args.Ack)
		}

		return 0, nil
	} else {
		var (
			total   int64
			eg, ctx = errgroup.WithContext(ctx)
		)

		message.Delay(n)

		for _, ep := range endpoints {
			addr := ep.Address()

			eg.Go(func() error {
				if v, err := l.doBroadcast(ctx, addr, args.Kind, args.Disconnect, message, args.Ack); err != nil {
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

// doBroadcast pushes a broadcast message to the gate at addr.
func (l *GateLinker) doBroadcast(ctx context.Context, addr string, kind session.Kind, disconnect bool, message buffer.Buffer, ack bool) (int64, error) {
	if client, err := l.builder.Build(addr); err != nil {
		message.Release()

		return 0, err
	} else {
		return client.Broadcast(ctx, kind, disconnect, message, ack)
	}
}

// Publish publishes a channel message to every gate.
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
			return l.doPublish(ctx, ep.Address(), args.Channel, args.Disconnect, message, args.Ack)
		}

		return 0, nil
	} else {
		var (
			total   int64
			eg, ctx = errgroup.WithContext(ctx)
		)

		message.Delay(n)

		for _, ep := range endpoints {
			addr := ep.Address()

			eg.Go(func() error {
				if v, err := l.doPublish(ctx, addr, args.Channel, args.Disconnect, message, args.Ack); err != nil {
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

// doPublish publishes a channel message to the gate at addr.
func (l *GateLinker) doPublish(ctx context.Context, addr string, channel string, disconnect bool, message buffer.Buffer, ack bool) (int64, error) {
	if client, err := l.builder.Build(addr); err != nil {
		message.Release()

		return 0, err
	} else {
		return client.Publish(ctx, channel, disconnect, message, ack)
	}
}

// Subscribe subscribes the targets to a channel.
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

// doDirectSubscribe subscribes the targets on the same gate server.
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

// doIndirectSubscribe subscribes each target on the gate that owns it.
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

// Unsubscribe unsubscribes the targets from a channel.
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

// doDirectUnsubscribe unsubscribes the targets on the same gate server.
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

// doIndirectUnsubscribe unsubscribes each target on the gate that owns it.
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

// doRPC locates the user's gate and invokes successHandler on it. It retries once when the
// located gate changes. failedHandler, when provided, is called before the call returns on every
// attempt that fails.
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

// doBuildClient builds a gate client for the given instance ID.
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

// PackMessage packs a message into a buffer, optionally encrypting its payload.
func (l *GateLinker) PackMessage(message *Message, encrypt bool) (buffer.Buffer, error) {
	buf, err := l.PackBuffer(message.Data, encrypt)
	if err != nil {
		return nil, err
	}

	return packet.PackMessage(&packet.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: buf,
	})
}

// PackBuffer encodes message, encrypting the encoded bytes when encrypt is true.
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

// WatchUserLocate watches user locate events and keeps the local source cache in sync.
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
				events, err := watcher.Next()
				if err != nil {
					if errors.Is(err, errors.ErrWatcherStopped) {
						return
					} else {
						continue
					}
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
		}
	}()
}

// WatchClusterInstance watches cluster instance changes and refreshes the dispatcher.
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
				services, err := watcher.Next()
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					} else {
						continue
					}
				}

				l.dispatcher.ReplaceServices(services...)
			}
		}
	}()
}
