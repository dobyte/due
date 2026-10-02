package node

import (
	"context"
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

// Proxy is a proxy.
//
// It provides the complete API of a node server, including gateway/node links, routing, events,
// message pushing and actor management.
type Proxy struct {
	node       *Node            // Node server
	gateLinker *link.GateLinker // Gateway linker
	nodeLinker *link.NodeLinker // Node linker
}

// newProxy creates a node proxy.
//
// It initializes the gateway linker and the node linker, reusing the codec, locator, registry and
// other options of the node.
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

// GetID returns the current node ID.
func (p *Proxy) GetID() string {
	return p.node.opts.id
}

// GetName returns the current node name.
func (p *Proxy) GetName() string {
	return p.node.opts.name
}

// GetState returns the current node state.
func (p *Proxy) GetState() cluster.State {
	return p.node.getState()
}

// SetState sets the current node state. It returns the error reported while setting the state.
func (p *Proxy) SetState(state cluster.State) error {
	return p.node.setState(state)
}

// Router returns the router.
func (p *Proxy) Router() *Router {
	return p.node.router
}

// RouteGroup returns a route group configured by the given functions.
func (p *Proxy) RouteGroup(groups ...func(group *RouterGroup)) *RouterGroup {
	return p.node.router.Group(groups...)
}

// Trigger returns the event trigger.
func (p *Proxy) Trigger() *Trigger {
	return p.node.trigger
}

// AddRouteHandler adds a route handler.
func (p *Proxy) AddRouteHandler(route int32, handler RouteHandler, opts ...RouteOptions) {
	p.node.router.AddRouteHandler(route, handler, opts...)
}

// SetDefaultRouteHandler sets the default route handler. Every unregistered route goes through the
// default route handler.
func (p *Proxy) SetDefaultRouteHandler(handler RouteHandler) {
	p.node.router.SetDefaultRouteHandler(handler)
}

// AddEventHandler adds an event handler.
func (p *Proxy) AddEventHandler(event cluster.Event, handler EventHandler) {
	p.node.trigger.addEventHandler(event, handler)
}

// AddHookListener adds a hook listener.
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.node.addHookListener(hook, handler)
}

// AddServiceProvider adds a service provider described by desc.
func (p *Proxy) AddServiceProvider(name string, desc, provider any) {
	p.node.addServiceProvider(name, desc, provider)
}

// NewMeshClient creates a new microservice client.
//
// target supports three modes:
//
//	service direct mode:    direct://127.0.0.1:8011
//	service direct mode:    direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
//	service discovery mode: discovery://service_name
//
// It returns the error reported when the node is shut down or no message transporter is configured.
func (p *Proxy) NewMeshClient(target string) (transport.Client, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	}

	if p.node.opts.transporter == nil {
		return nil, errors.ErrMissingTransporter
	}

	return p.node.opts.transporter.NewClient(target)
}

// HasGate reports whether the given gateway exists. It returns an error when the node is shut down.
func (p *Proxy) HasGate(gid string) (bool, error) {
	if p.node.isShut() {
		return false, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.HasGate(gid), nil
	}
}

// AskGate reports whether the user is on the given gateway. It returns the gateway ID the user
// actually resides on, whether the user is on the given gateway, and an error when the node is shut
// down.
func (p *Proxy) AskGate(ctx context.Context, gid string, uid int64) (string, bool, error) {
	if p.node.isShut() {
		return "", false, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.AskGate(ctx, gid, uid)
	}
}

// LocateGate locates the gateway the user resides on. It returns the gateway ID and an error when
// the node is shut down.
func (p *Proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	if p.node.isShut() {
		return "", errors.ErrNodeShutdown
	} else {
		return p.gateLinker.LocateGate(ctx, uid)
	}
}

// BindGate binds the gateway. Once bound, the user is associated with the connection. It returns the
// error reported when the node is shut down or the binding fails.
func (p *Proxy) BindGate(ctx context.Context, gid string, cid, uid int64) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.BindGate(ctx, gid, cid, uid)
	}
}

// UnbindGate unbinds the gateway. It returns the error reported when the node is shut down or the
// unbinding fails.
func (p *Proxy) UnbindGate(ctx context.Context, uid int64) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.UnbindGate(ctx, uid)
	}
}

// FetchGateList fetches the gateway list filtered by states. It returns the gateway service
// instances and an error when the node is shut down.
func (p *Proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.FetchGateList(ctx, states...)
	}
}

// HasNode reports whether the given node exists.
func (p *Proxy) HasNode(nid string) bool {
	return p.nodeLinker.HasNode(nid)
}

// AskNode reports whether the user is on the given node. It returns the node ID the user actually
// resides on, whether the user is on the given node, and an error when the node is shut down.
func (p *Proxy) AskNode(ctx context.Context, uid int64, name, nid string) (string, bool, error) {
	if p.node.isShut() {
		return "", false, errors.ErrNodeShutdown
	} else {
		return p.nodeLinker.AskNode(ctx, uid, name, nid)
	}
}

// LocateNode locates the node the user resides on. It returns the node ID and an error when the node
// is shut down.
func (p *Proxy) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	if p.node.isShut() {
		return "", errors.ErrNodeShutdown
	} else {
		return p.nodeLinker.LocateNode(ctx, uid, name)
	}
}

// LocateNodes locates the nodes the user resides on. It returns a map from node name to node ID the
// user is bound to, and an error when the node is shut down.
func (p *Proxy) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	} else {
		return p.nodeLinker.LocateNodes(ctx, uid)
	}
}

// BindNode binds the node.
//
// A user can be bound to multiple node servers, but only one node server per name; binding to a node
// server with the same name again overrides the previous binding. The binding is synchronized to the
// gateway servers and other related node servers through publish and subscribe. It returns the error
// reported when the node is shut down or the binding fails.
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

// UnbindNode unbinds the node.
//
// It unbinds the node server with the corresponding name and verifies the node ID, so the unbinding
// fails on a mismatch. The unbinding is synchronized to the gateway servers and other related node
// servers through publish and subscribe. It returns the error reported when the node is shut down or
// the unbinding fails.
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

// FetchNodeList fetches the node list filtered by states. It returns the node service instances and
// an error when the node is shut down.
func (p *Proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	} else {
		return p.nodeLinker.FetchNodeList(ctx, states...)
	}
}

// BindActor binds an actor. It returns the error reported when the node is shut down or the binding
// fails.
func (p *Proxy) BindActor(uid int64, kind, id string) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.node.scheduler.bindActor(uid, kind, id)
	}
}

// UnbindActor unbinds an actor. It returns the error reported when the node is shut down or the
// unbinding fails.
func (p *Proxy) UnbindActor(uid int64, kind string) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.node.scheduler.unbindActor(uid, kind)
	}
}

// PackMessage packs a message. It returns the packed bytes and the error reported when packing
// fails.
func (p *Proxy) PackMessage(message *cluster.Message) ([]byte, error) {
	buf, err := p.gateLinker.PackMessage(message, true)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// PackBuffer packs the given message content. It returns the packed bytes and the error reported
// when packing fails.
func (p *Proxy) PackBuffer(message any) ([]byte, error) {
	return p.gateLinker.PackBuffer(message, true)
}

// GetIP returns the client IP. It returns the error reported when the node is shut down or the query
// fails.
func (p *Proxy) GetIP(ctx context.Context, args *cluster.GetIPArgs) (string, error) {
	if p.node.isShut() {
		return "", errors.ErrNodeShutdown
	} else {
		return p.gateLinker.GetIP(ctx, args)
	}
}

// Stat counts the sessions of the given kind. It returns the session count and an error when the
// node is shut down.
func (p *Proxy) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	if p.node.isShut() {
		return 0, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Stat(ctx, kind)
	}
}

// IsOnline reports whether the target is online. It returns the error reported when the node is shut
// down or the query fails.
func (p *Proxy) IsOnline(ctx context.Context, args *cluster.IsOnlineArgs) (bool, error) {
	if p.node.isShut() {
		return false, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.IsOnline(ctx, args)
	}
}

// Disconnect disconnects a connection. It returns the error reported when the node is shut down or
// the disconnection fails.
func (p *Proxy) Disconnect(ctx context.Context, args *cluster.DisconnectArgs) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Disconnect(ctx, args)
	}
}

// Push pushes a message.
//
// Set args.Ack to true to get the actual sending result of the message. It returns the error
// reported when the node is shut down or the push fails.
func (p *Proxy) Push(ctx context.Context, args *cluster.PushArgs) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Push(ctx, args)
	}
}

// Multicast pushes a multicast message.
//
// Set args.Ack to true to get the number of targets the push succeeded on. It returns that number
// and the error reported when the node is shut down or the push fails.
func (p *Proxy) Multicast(ctx context.Context, args *cluster.MulticastArgs) (int64, error) {
	if p.node.isShut() {
		return 0, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Multicast(ctx, args)
	}
}

// Broadcast pushes a broadcast message.
//
// Set args.Ack to true to get the number of targets the push succeeded on. It returns that number
// and the error reported when the node is shut down or the push fails.
func (p *Proxy) Broadcast(ctx context.Context, args *cluster.BroadcastArgs) (int64, error) {
	if p.node.isShut() {
		return 0, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Broadcast(ctx, args)
	}
}

// Publish publishes a message.
//
// Set args.Ack to true to get the number of targets the publish succeeded on. It returns that number
// and the error reported when the node is shut down or the publish fails.
func (p *Proxy) Publish(ctx context.Context, args *cluster.PublishArgs) (int64, error) {
	if p.node.isShut() {
		return 0, errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Publish(ctx, args)
	}
}

// Subscribe subscribes to a channel. It returns the error reported when the node is shut down or the
// subscription fails.
func (p *Proxy) Subscribe(ctx context.Context, args *cluster.SubscribeArgs) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Subscribe(ctx, args)
	}
}

// Unsubscribe unsubscribes from a channel. It returns the error reported when the node is shut down
// or the unsubscription fails.
func (p *Proxy) Unsubscribe(ctx context.Context, args *cluster.UnsubscribeArgs) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	} else {
		return p.gateLinker.Unsubscribe(ctx, args)
	}
}

// Deliver delivers a message to a node for handling. It returns the error reported when the node is
// shut down, the target is the current node or the delivery fails.
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

// Invoke calls a function in a thread-safe way.
//
// A synchronous call (wait=true) issued from within the node dispatch goroutine executes the
// function directly, avoiding a deadlock while waiting for the queue it runs in. Do not wait
// synchronously across queues in a dispatch chain (for example, a node task synchronously waiting
// for an actor task to finish while the actor task is synchronously waiting for the node task); the
// mutual wait forms a cross-queue circular wait and deadlocks. It returns the error reported when
// the node is shut down or the task fails to be enqueued.
func (p *Proxy) Invoke(f func(), wait ...bool) error {
	if p.node.isShut() {
		return errors.ErrNodeShutdown
	}

	if len(wait) > 0 && wait[0] && p.node.dispatchGoid.Load() == goid.Get() {
		xcall.Call(f)
	} else {
		if !p.node.doAddWait() {
			return errors.ErrNodeShutdown
		}

		wg, err := p.node.tasker.Commit(f, wait...)
		if err != nil {
			p.node.doDoneWait()
			return err
		}

		if wg != nil {
			wg.Wait()
		}
	}

	return nil
}

// AfterFunc schedules a delayed call and is used the same way as [time.AfterFunc]. It returns a
// timer that can be cancelled with Stop, or an error when the node is shut down.
func (p *Proxy) AfterFunc(d time.Duration, f func()) (*Timer, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	}

	if !p.node.doAddWait() {
		return nil, errors.ErrNodeShutdown
	}

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

// AfterInvoke schedules a thread-safe delayed call.
//
// The function is executed serially through the task queue after the delay, which guarantees thread
// safety. It returns a timer that can be cancelled with Stop, or an error when the node is shut
// down.
func (p *Proxy) AfterInvoke(d time.Duration, f func()) (*Timer, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	}

	if !p.node.doAddWait() {
		return nil, errors.ErrNodeShutdown
	}

	timer := time.AfterFunc(d, func() {
		var err error

		if p.node.isShut() {
			err = errors.ErrNodeShutdown
		} else {
			_, err = p.node.tasker.Commit(f)
		}

		if err != nil {
			log.Warnf("node after invoke failed: %v", err)
			p.node.doDoneWait()
		}
	})

	return &Timer{node: p.node, timer: timer}, nil
}

// Spawn creates a new actor. It returns the error reported when the node is shut down or the actor
// fails to be created.
func (p *Proxy) Spawn(creator Creator, opts ...ActorOption) (*Actor, error) {
	if p.node.isShut() {
		return nil, errors.ErrNodeShutdown
	} else {
		return p.node.scheduler.spawn(creator, opts...)
	}
}

// Kill kills an existing actor. It reports whether the actor was killed successfully, and returns
// false when the node is shut down or the actor does not exist.
func (p *Proxy) Kill(kind, id string) bool {
	if p.node.isShut() {
		return false
	} else {
		return p.node.scheduler.kill(kind, id)
	}
}

// Actor returns an actor and whether it exists.
func (p *Proxy) Actor(kind, id string) (*Actor, bool) {
	return p.node.scheduler.load(kind, id)
}

// watch starts watching.
//
// It watches user location and cluster instance changes; both the gateway linker and the node linker
// subscribe to the related changes.
func (p *Proxy) watch() {
	p.gateLinker.WatchUserLocate()

	p.gateLinker.WatchClusterInstance()

	p.nodeLinker.WatchUserLocate()

	p.nodeLinker.WatchClusterInstance()
}
