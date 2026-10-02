package link

import (
	"context"
	"sync"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/internal/dispatcher"
	"github.com/dobyte/due/v2/internal/transporter/node"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/registry"
	"golang.org/x/sync/errgroup"
)

// sourceShardNum is the number of shards in the user source node cache. It is a power of two so
// that a bitwise AND can select a shard.
const sourceShardNum = 256

// sourceShard is one shard of the user source node cache.
//
// Shards are selected by hashing the low bits of the UID to reduce contention on the read-write
// lock. The trailing padding fills the struct up to a 64-byte cache line so that adjacent shards
// do not suffer from false sharing.
type sourceShard struct {
	rw      sync.RWMutex
	sources map[int64]map[string]string
	_       [32]byte
}

type NodeLinker struct {
	ctx        context.Context             // Context
	opts       *Options                    // Options
	builder    *node.Builder               // Builder
	dispatcher *dispatcher.Dispatcher      // Dispatcher
	shards     [sourceShardNum]sourceShard // User source node cache shards
}

func NewNodeLinker(ctx context.Context, opts *Options) *NodeLinker {
	l := &NodeLinker{
		ctx:        ctx,
		opts:       opts,
		dispatcher: dispatcher.NewDispatcher(opts.Dispatch),
		builder: node.NewBuilder(&node.ClientOptions{
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

	for i := range l.shards {
		l.shards[i].sources = make(map[int64]map[string]string)
	}

	return l
}

// shard returns the source cache shard that owns uid.
func (l *NodeLinker) shard(uid int64) *sourceShard {
	return &l.shards[uint64(uid)&(sourceShardNum-1)]
}

// HasNode reports whether the node identified by nid exists.
func (l *NodeLinker) HasNode(nid string) bool {
	_, err := l.dispatcher.FindEndpoint(nid)
	return err == nil
}

// AskNode reports whether the user is located on the node with the given name and instance ID
// nid.
func (l *NodeLinker) AskNode(ctx context.Context, uid int64, name, nid string) (string, bool, error) {
	if l.opts.Locator == nil {
		return "", false, errors.ErrNotFoundLocator
	}

	if insID, ok := l.doLoadSource(uid, name); ok {
		return insID, insID == nid, nil
	}

	insID, err := l.opts.Locator.LocateNode(ctx, uid, name)
	if err != nil {
		return "", false, err
	}

	if insID == "" {
		return "", false, errors.ErrNotFoundUserLocation
	}

	l.doStoreSource(uid, name, insID)

	return insID, insID == nid, nil
}

// LocateNode returns the node where the user is located.
func (l *NodeLinker) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	if l.opts.Locator == nil {
		return "", errors.ErrNotFoundLocator
	}

	nid, ok := l.doLoadSource(uid, name)
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

	l.doStoreSource(uid, name, nid)

	return nid, nil
}

// LocateNodes returns the list of nodes where the user is located.
func (l *NodeLinker) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	if l.opts.Locator == nil {
		return nil, errors.ErrNotFoundLocator
	}

	return l.opts.Locator.LocateNodes(ctx, uid)
}

// BindNode binds the user to the node with the given name and instance ID nid.
//
// A user may be bound to multiple node servers, but only one server per name. Binding to a server
// with the same name again overwrites the previous binding. The binding is propagated to the gate
// servers and other related node servers through publish-subscribe.
func (l *NodeLinker) BindNode(ctx context.Context, uid int64, name, nid string) error {
	if l.opts.Locator == nil {
		return errors.ErrNotFoundLocator
	}

	if err := l.opts.Locator.BindNode(ctx, uid, name, nid); err != nil {
		return err
	}

	l.doStoreSource(uid, name, nid)

	return nil
}

// UnbindNode unbinds the user from the node with the given name.
//
// The node instance ID is verified while unbinding, and the unbind fails when nid does not match.
// The unbind is propagated to the gate servers and other related node servers through
// publish-subscribe.
func (l *NodeLinker) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	if l.opts.Locator == nil {
		return errors.ErrNotFoundLocator
	}

	if err := l.opts.Locator.UnbindNode(ctx, uid, name, nid); err != nil {
		return err
	}

	l.doDeleteSource(uid, name, nid)

	return nil
}

// FetchNodeList returns the node services, optionally filtered by states.
func (l *NodeLinker) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	services, err := l.opts.Registry.Services(ctx, cluster.Node.String())
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

// Deliver delivers a message to a node for handling.
func (l *NodeLinker) Deliver(ctx context.Context, args *DeliverArgs) error {
	var (
		err       error
		buf       buffer.Buffer
		isDeliver bool
	)

	switch b := args.Buffer.(type) {
	case []byte:
		buf = buffer.NewBytes(b)
	case buffer.Buffer:
		buf = b
	case *Message:
		if buf, err = l.PackMessage(b, false); err != nil {
			return err
		}
	default:
		return errors.ErrInvalidMessage
	}

	if args.NID != "" {
		client, err := l.doBuildClient(args.NID)
		if err != nil {
			buf.Release()
			return err
		} else {
			return client.Deliver(ctx, args.CID, args.UID, buf)
		}
	} else {
		if _, err = l.doRPC(ctx, args.Route, args.UID, func(ctx context.Context, client *node.Client) (bool, any, error) {
			isDeliver = true

			return false, nil, client.Deliver(ctx, args.CID, args.UID, buf)
		}); err != nil {
			if !isDeliver {
				buf.Release()
			}

			if !errors.Is(err, errors.ErrNotFoundUserLocation) {
				return err
			}
		}

		return nil
	}
}

// Trigger triggers an event on every node subscribed to it.
//
// Events are triggered on nodes in parallel, so that dialing or a blocked write queue on one node
// does not slow down delivery to the others. The call waits in the background for all nodes to
// finish and releases the context derived by the errgroup to avoid a context leak. Failures on
// individual nodes are logged at the error level rather than requiring the remaining nodes to
// complete first.
func (l *NodeLinker) Trigger(ctx context.Context, args *TriggerArgs) error {
	event, err := l.dispatcher.FindEvent(int(args.Event))
	if err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)

	event.VisitEndpoints(func(insID string, ep *endpoint.Endpoint) bool {
		eg.Go(func() error {
			client, err := l.builder.Build(ep.Address())
			if err != nil {
				log.Errorf("build node client failed, nid: %s, addr: %s, event: %v, cid: %d, uid: %d, err: %v", insID, ep.Address(), args.Event, args.CID, args.UID, err)
				return nil
			}

			if err = client.Trigger(ctx, args.Event, args.CID, args.UID); err != nil {
				switch {
				case errors.Is(err, errors.ErrConnectionClosed), errors.Is(err, errors.ErrConnectionHanged):
					log.Warnf("trigger event failed, nid: %s, event: %v, cid: %d, uid: %d, err: %v", insID, args.Event, args.CID, args.UID, err)
				default:
					log.Errorf("trigger event failed, nid: %s, event: %v, cid: %d, uid: %d, err: %v", insID, args.Event, args.CID, args.UID, err)
				}
			}

			return nil
		})

		return true
	})

	return eg.Wait()
}

// GetState returns the state of the node identified by nid.
func (l *NodeLinker) GetState(ctx context.Context, nid string) (cluster.State, error) {
	client, err := l.doBuildClient(nid)
	if err != nil {
		return cluster.Shut, err
	}

	return client.GetState(ctx)
}

// SetState sets the state of the node identified by nid.
func (l *NodeLinker) SetState(ctx context.Context, nid string, state cluster.State) error {
	client, err := l.doBuildClient(nid)
	if err != nil {
		return err
	}

	return client.SetState(ctx, state)
}

// doRPC performs an RPC call against a node selected by the given route.
func (l *NodeLinker) doRPC(ctx context.Context, routeID int32, uid int64, fn func(ctx context.Context, client *node.Client) (bool, any, error)) (any, error) {
	var (
		err       error
		nid       string
		prev      string
		route     *dispatcher.Route
		client    *node.Client
		ep        *endpoint.Endpoint
		continued bool
		reply     any
	)

	if route, err = l.dispatcher.FindRoute(routeID); err != nil {
		return nil, err
	}

	if uid == 0 && (route.Stateful() || route.Authorized()) {
		return nil, errors.ErrIllegalRequest
	}

	if l.opts.Kind == cluster.Gate && route.Internal() {
		return nil, errors.ErrIllegalRequest
	}

	for range 2 {
		if route.Stateful() {
			if nid, err = l.LocateNode(ctx, uid, route.Group()); err != nil {
				return nil, err
			}

			if nid == prev {
				return reply, err
			}

			prev = nid
		}

		if ep, err = route.FindEndpoint(nid); err != nil {
			return nil, err
		}

		if client, err = l.builder.Build(ep.Address()); err != nil {
			return nil, err
		}

		if continued, reply, err = fn(ctx, client); continued {
			if route.Stateful() {
				l.doDeleteSource(uid, route.Group(), prev)
			}
			continue
		}

		break
	}

	return reply, err
}

// doBuildClient builds a node client for the given instance ID.
func (l *NodeLinker) doBuildClient(nid string) (*node.Client, error) {
	if nid == "" {
		return nil, errors.ErrInvalidNID
	}

	ep, err := l.dispatcher.FindEndpoint(nid)
	if err != nil {
		return nil, err
	}

	return l.builder.Build(ep.Address())
}

// PackMessage packs a message into a buffer, optionally encrypting its payload.
func (l *NodeLinker) PackMessage(message *Message, encrypt bool) (buffer.Buffer, error) {
	buffer, err := l.doPackBuffer(message.Data, encrypt)
	if err != nil {
		return nil, err
	}

	return packet.PackMessage(&packet.Message{
		Seq:    message.Seq,
		Route:  message.Route,
		Buffer: buffer,
	})
}

// doPackBuffer encodes message, encrypting the encoded bytes when encrypt is true.
func (l *NodeLinker) doPackBuffer(message any, encrypt bool) ([]byte, error) {
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

// doStoreSource stores the node source of the user.
func (l *NodeLinker) doStoreSource(uid int64, name, nid string) {
	sh := l.shard(uid)

	wait, done := func() (bool, bool) {
		sh.rw.Lock()
		defer sh.rw.Unlock()

		if sources, ok := sh.sources[uid]; ok {
			if oldNID, ok := sources[name]; ok {
				if oldNID == nid {
					return false, false
				} else {
					sources[name] = nid

					switch l.opts.ID {
					case oldNID:
						return false, true
					case nid:
						return true, false
					default:
						return false, false
					}
				}
			} else {
				sources[name] = nid

				return l.opts.ID == nid, false
			}
		} else {
			sh.sources[uid] = map[string]string{name: nid}

			return l.opts.ID == nid, false
		}
	}()

	if wait && l.opts.WaitHandler != nil {
		l.opts.WaitHandler()
	}

	if done && l.opts.DoneHandler != nil {
		l.opts.DoneHandler()
	}
}

// doDeleteSource removes the node source of the user.
func (l *NodeLinker) doDeleteSource(uid int64, name, nid string) {
	sh := l.shard(uid)

	done := func() bool {
		sh.rw.Lock()
		defer sh.rw.Unlock()

		sources, ok := sh.sources[uid]
		if !ok {
			return false
		}

		oldNID, ok := sources[name]
		if !ok {
			return false
		}

		// ignore mismatched NID
		if oldNID != nid {
			return false
		}

		if len(sources) == 1 {
			delete(sh.sources, uid)
		} else {
			delete(sources, name)
		}

		return oldNID == l.opts.ID
	}()

	if done && l.opts.DoneHandler != nil {
		l.opts.DoneHandler()
	}
}

// doLoadSource loads the node source of the user.
//
// It is called for every stateful message delivery. It takes the read lock on the shard selected
// by UID, avoiding the cache-line contention a global lock would cause under high-concurrency
// delivery.
func (l *NodeLinker) doLoadSource(uid int64, name string) (string, bool) {
	sh := l.shard(uid)

	sh.rw.RLock()
	sources, ok := sh.sources[uid]
	if ok {
		if nid, ok := sources[name]; ok {
			sh.rw.RUnlock()
			return nid, true
		}
	}
	sh.rw.RUnlock()

	return "", false
}

// WatchUserLocate watches user locate events and keeps the local source cache in sync.
func (l *NodeLinker) WatchUserLocate() {
	if l.opts.Locator == nil {
		return
	}

	ctx, cancel := context.WithTimeout(l.ctx, 3*time.Second)
	watcher, err := l.opts.Locator.Watch(ctx, cluster.Node.String())
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
					case locate.BindNode:
						l.doStoreSource(event.UID, event.InsName, event.InsID)
					case locate.UnbindNode:
						l.doDeleteSource(event.UID, event.InsName, event.InsID)
					default:
						// ignore
					}
				}
			}
		}
	}()
}

// WatchClusterInstance watches cluster instance changes and refreshes the dispatcher.
func (l *NodeLinker) WatchClusterInstance() {
	ctx, cancel := context.WithTimeout(l.ctx, 3*time.Second)
	watcher, err := l.opts.Registry.Watch(ctx, cluster.Node.String())
	cancel()
	if err != nil {
		log.Fatalf("the cluster instance watch failed: %v", err)
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
