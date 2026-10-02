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

// Proxy is the mesh proxy.
//
// It provides the complete API exposed by the mesh server, including gate/node linking and message
// pushing.
type Proxy struct {
	mesh       *Mesh            // mesh server
	gateLinker *link.GateLinker // gate linker
	nodeLinker *link.NodeLinker // node linker
}

// newProxy creates a mesh proxy.
//
// It initializes the gate linker and the node linker, reusing the mesh's codec, locator, registry
// and other options underneath.
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

// GetID returns the current instance ID.
func (p *Proxy) GetID() string {
	return p.mesh.opts.id
}

// GetName returns the current instance name.
func (p *Proxy) GetName() string {
	return p.mesh.opts.name
}

// GetState returns the current mesh state.
func (p *Proxy) GetState() cluster.State {
	return p.mesh.getState()
}

// SetState sets the current mesh state, returning an error when the state cannot be set.
func (p *Proxy) SetState(state cluster.State) error {
	return p.mesh.setState(state)
}

// AddServiceProvider adds a service provider. The name is the service name, desc is the service
// description and provider is the service provider.
func (p *Proxy) AddServiceProvider(name string, desc, provider any) {
	p.mesh.addServiceProvider(name, desc, provider)
}

// AddHookListener adds a hook listener for the given hook.
func (p *Proxy) AddHookListener(hook cluster.Hook, handler HookHandler) {
	p.mesh.addHookListener(hook, handler)
}

// NewMeshClient creates a new mesh client.
//
// The target parameter supports three forms:
//
//	direct://127.0.0.1:8011                        (direct connection by address)
//	direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070  (direct connection by instance ID)
//	discovery://service_name                       (service discovery by service name)
//
// It returns an error when the mesh has been shut down.
func (p *Proxy) NewMeshClient(target string) (transport.Client, error) {
	if p.mesh.isShut() {
		return nil, errors.ErrMeshShutdown
	} else {
		return p.mesh.opts.transporter.NewClient(target)
	}
}

// HasGate reports whether the given gate exists. It returns an error when the mesh has been shut
// down.
func (p *Proxy) HasGate(gid string) (bool, error) {
	if p.mesh.isShut() {
		return false, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.HasGate(gid), nil
	}
}

// AskGate reports whether the user is on the given gate. It returns the gate ID the user is
// actually on, whether the user is on the given gate, and an error when the mesh has been shut
// down.
func (p *Proxy) AskGate(ctx context.Context, gid string, uid int64) (string, bool, error) {
	if p.mesh.isShut() {
		return "", false, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.AskGate(ctx, gid, uid)
	}
}

// LocateGate locates the gate the user is on and returns its gate ID, or an error when the mesh
// has been shut down.
func (p *Proxy) LocateGate(ctx context.Context, uid int64) (string, error) {
	if p.mesh.isShut() {
		return "", errors.ErrMeshShutdown
	} else {
		return p.gateLinker.LocateGate(ctx, uid)
	}
}

// BindGate binds the gate, associating the user with the connection. It returns an error when the
// mesh has been shut down or binding fails.
func (p *Proxy) BindGate(ctx context.Context, gid string, cid, uid int64) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.BindGate(ctx, gid, cid, uid)
	}
}

// UnbindGate unbinds the gate. It returns an error when the mesh has been shut down or unbinding
// fails.
func (p *Proxy) UnbindGate(ctx context.Context, uid int64) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.UnbindGate(ctx, uid)
	}
}

// FetchGateList fetches the gate list filtered by the given states, or an error when the mesh has
// been shut down.
func (p *Proxy) FetchGateList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	if p.mesh.isShut() {
		return nil, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.FetchGateList(ctx, states...)
	}
}

// HasNode reports whether the given node exists. It returns an error when the mesh has been shut
// down.
func (p *Proxy) HasNode(nid string) (bool, error) {
	if p.mesh.isShut() {
		return false, errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.HasNode(nid), nil
	}
}

// AskNode reports whether the user is on the given node. It returns the node ID the user is
// actually on, whether the user is on the given node, and an error when the mesh has been shut
// down.
func (p *Proxy) AskNode(ctx context.Context, uid int64, name, nid string) (string, bool, error) {
	if p.mesh.isShut() {
		return "", false, errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.AskNode(ctx, uid, name, nid)
	}
}

// LocateNode locates the node the user is on and returns its node ID, or an error when the mesh
// has been shut down.
func (p *Proxy) LocateNode(ctx context.Context, uid int64, name string) (string, error) {
	if p.mesh.isShut() {
		return "", errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.LocateNode(ctx, uid, name)
	}
}

// LocateNodes locates the nodes the user is on and returns a map from node name to node ID, or an
// error when the mesh has been shut down.
func (p *Proxy) LocateNodes(ctx context.Context, uid int64) (map[string]string, error) {
	if p.mesh.isShut() {
		return nil, errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.LocateNodes(ctx, uid)
	}
}

// BindNode binds a node.
//
// A single user may bind to multiple node servers, but only one node server per name; binding to a
// node server with the same name multiple times overwrites the previous binding. The binding is
// synchronized to the gate server and other related node servers through publish-subscribe. It
// returns an error when the mesh has been shut down or binding fails.
func (p *Proxy) BindNode(ctx context.Context, uid int64, name, nid string) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.BindNode(ctx, uid, name, nid)
	}
}

// UnbindNode unbinds a node.
//
// It unbinds the node server with the corresponding name and validates the node ID; the unbind
// fails when the node ID does not match. The unbind is synchronized to the gate server and other
// related node servers through publish-subscribe. It returns an error when the mesh has been shut
// down or unbinding fails.
func (p *Proxy) UnbindNode(ctx context.Context, uid int64, name, nid string) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.UnbindNode(ctx, uid, name, nid)
	}
}

// FetchNodeList fetches the node list filtered by the given states, or an error when the mesh has
// been shut down.
func (p *Proxy) FetchNodeList(ctx context.Context, states ...cluster.State) ([]*registry.ServiceInstance, error) {
	if p.mesh.isShut() {
		return nil, errors.ErrMeshShutdown
	} else {
		return p.nodeLinker.FetchNodeList(ctx, states...)
	}
}

// PackMessage packs the given message and returns the packed message bytes, or an error when
// packing fails.
func (p *Proxy) PackMessage(message *cluster.Message) ([]byte, error) {
	buf, err := p.gateLinker.PackMessage(message, true)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// PackBuffer packs the given message content and returns the packed message bytes, or an error
// when packing fails.
func (p *Proxy) PackBuffer(message any) ([]byte, error) {
	return p.gateLinker.PackBuffer(message, true)
}

// GetIP returns the client IP, or an error when the mesh has been shut down or the query fails.
func (p *Proxy) GetIP(ctx context.Context, args *cluster.GetIPArgs) (string, error) {
	if p.mesh.isShut() {
		return "", errors.ErrMeshShutdown
	} else {
		return p.gateLinker.GetIP(ctx, args)
	}
}

// Stat returns the total number of sessions, or an error when the mesh has been shut down.
func (p *Proxy) Stat(ctx context.Context, kind session.Kind) (int64, error) {
	if p.mesh.isShut() {
		return 0, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Stat(ctx, kind)
	}
}

// IsOnline reports whether the session is online, or an error when the mesh has been shut down or
// the query fails.
func (p *Proxy) IsOnline(ctx context.Context, args *cluster.IsOnlineArgs) (bool, error) {
	if p.mesh.isShut() {
		return false, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.IsOnline(ctx, args)
	}
}

// Disconnect disconnects the session, or returns an error when the mesh has been shut down or
// disconnecting fails.
func (p *Proxy) Disconnect(ctx context.Context, args *cluster.DisconnectArgs) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Disconnect(ctx, args)
	}
}

// Push pushes a message. Setting args.Ack to true reports the actual delivery result. It returns
// an error when the mesh has been shut down or pushing fails.
func (p *Proxy) Push(ctx context.Context, args *cluster.PushArgs) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Push(ctx, args)
	}
}

// Multicast pushes a multicast message and returns the number of targets pushed successfully, or
// an error when the mesh has been shut down or pushing fails. To get the number of successful
// targets, set args.Ack to true.
func (p *Proxy) Multicast(ctx context.Context, args *cluster.MulticastArgs) (int64, error) {
	if p.mesh.isShut() {
		return 0, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Multicast(ctx, args)
	}
}

// Broadcast pushes a broadcast message and returns the number of targets pushed successfully, or
// an error when the mesh has been shut down or pushing fails. To get the number of successful
// targets, set args.Ack to true.
func (p *Proxy) Broadcast(ctx context.Context, args *cluster.BroadcastArgs) (int64, error) {
	if p.mesh.isShut() {
		return 0, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Broadcast(ctx, args)
	}
}

// Publish publishes a message and returns the number of targets published successfully, or an
// error when the mesh has been shut down or publishing fails. To get the number of successful
// targets, set args.Ack to true.
func (p *Proxy) Publish(ctx context.Context, args *cluster.PublishArgs) (int64, error) {
	if p.mesh.isShut() {
		return 0, errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Publish(ctx, args)
	}
}

// Subscribe subscribes to a channel, or returns an error when the mesh has been shut down or
// subscribing fails.
func (p *Proxy) Subscribe(ctx context.Context, args *cluster.SubscribeArgs) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Subscribe(ctx, args)
	}
}

// Unsubscribe unsubscribes from a channel, or returns an error when the mesh has been shut down or
// unsubscribing fails.
func (p *Proxy) Unsubscribe(ctx context.Context, args *cluster.UnsubscribeArgs) error {
	if p.mesh.isShut() {
		return errors.ErrMeshShutdown
	} else {
		return p.gateLinker.Unsubscribe(ctx, args)
	}
}

// Deliver delivers a message to a node for processing, or returns an error when the mesh has been
// shut down or delivering fails.
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

// watch starts watching user locations and cluster instance changes; both the gate linker and the
// node linker subscribe to the related changes.
func (p *Proxy) watch() {
	p.gateLinker.WatchUserLocate()

	p.gateLinker.WatchClusterInstance()

	p.nodeLinker.WatchUserLocate()

	p.nodeLinker.WatchClusterInstance()
}
