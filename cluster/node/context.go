package node

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/transport"
)

// Context is a message context.
//
// It wraps the basic information about the gateway, node, connection and user, and provides a
// series of methods such as route handling, message delivery and actor operations.
type Context interface {
	// GID returns the ID of the source gateway.
	GID() string
	// NID returns the ID of the source node.
	NID() string
	// CID returns the connection ID.
	CID() int64
	// UID returns the user ID.
	UID() int64
	// Seq returns the message sequence number.
	Seq() int32
	// Route returns the message route number.
	Route() int32
	// Event returns the event type.
	Event() cluster.Event
	// Kind returns the message kind, either an event or a request.
	Kind() Kind
	// Parse parses the message.
	//
	// By default it deserializes into the target value; when an encryptor is present and the
	// message comes from a gateway, it decrypts the message before deserializing. It reports an
	// error when decryption or deserialization fails.
	Parse(v any) error
	// Defer adds a deferred call to the call stack.
	//
	// This method behaves like the go defer statement and its scope is limited to the current
	// handler function, so it is recommended over go defer. The difference is that Defer allows
	// the call stack to be cancelled. The stack is cancelled automatically when Task or Next is
	// called, and it can also be cancelled manually with Cancel. The optional bottom controls
	// whether fn is pushed to the bottom of the stack; it is pushed to the top by default.
	Defer(fn func(), bottom ...bool)
	// Cancel cancels the defer call stack.
	Cancel()
	// Clone clones the context.
	//
	// It creates a new independent context, which is used to deliver the message to another actor.
	Clone() Context
	// Task submits a task.
	//
	// It is recommended over task.Add and go func; calling it automatically cancels every function
	// in the defer call stack.
	Task(fn func(ctx Context))
	// Proxy returns the node proxy.
	Proxy() *Proxy
	// Context returns the underlying standard context.
	Context() context.Context
	// SetValue sets a value on the context.
	SetValue(key, val any)
	// GetValue returns the value associated with key, or nil when it does not exist.
	GetValue(key any) any
	// GetIP returns the client IP. It reports an error when the source is illegal or the IP cannot
	// be retrieved.
	GetIP() (string, error)
	// Deliver delivers a message to a node for processing.
	Deliver(args *cluster.DeliverArgs) error
	// Reply replies with a message.
	//
	// According to the message source (gateway, actor or node), the message is sent back to the
	// corresponding target.
	Reply(message *cluster.Message) error
	// Response responds to the current route message.
	//
	// The response reuses the route number and sequence number of the original request.
	Response(message any) error
	// Disconnect closes the connection from the gateway. The optional force controls whether the
	// connection is disconnected forcibly; it is not forced by default.
	Disconnect(force ...bool) error
	// BindGate binds the gateway. When uid is not specified, the uid of the current context user is
	// used.
	BindGate(uid ...int64) error
	// UnbindGate unbinds the gateway. When uid is not specified, the uid of the current context
	// user is used.
	UnbindGate(uid ...int64) error
	// BindNode binds the node. When uid is not specified, the uid of the current context user is
	// used.
	BindNode(uid ...int64) error
	// UnbindNode unbinds the node. When uid is not specified, the uid of the current context user
	// is used.
	UnbindNode(uid ...int64) error
	// Subscribe subscribes to a channel. When uids are specified, it subscribes the user channels;
	// otherwise it subscribes the channel of the current connection.
	Subscribe(channel string, uids ...int64) error
	// Unsubscribe unsubscribes from a channel. When uids are specified, it unsubscribes the user
	// channels; otherwise it unsubscribes the channel of the current connection.
	Unsubscribe(channel string, uids ...int64) error
	// BindActor binds an actor, establishing a binding between the current user and the given
	// actor.
	BindActor(kind, id string) error
	// UnbindActor unbinds the actor of the given kind.
	UnbindActor(kind string)
	// Next dispatches the message down to the node.
	//
	// It hands the current message to the node's scheduler for a second dispatch. Calling it
	// automatically cancels every function in the defer call stack.
	Next() error
	// Spawn derives a new actor.
	//
	// creator creates the actor processor and opts configures the new actor. It reports an error
	// when an actor with the same kind and ID already exists or when creation fails.
	Spawn(creator Creator, opts ...ActorOption) (*Actor, error)
	// Kill kills an existing actor, reporting whether it was killed successfully.
	Kill(kind, id string) bool
	// Actor returns the actor with the given kind and ID, and whether it exists.
	Actor(kind, id string) (*Actor, bool)
	// Invoke calls a function in a thread-safe manner.
	//
	// Inside a global handler, it calls proxy.Invoke; inside an actor processor, it calls
	// actor.Invoke. The optional isBlock controls whether the call is blocking; it is non-blocking
	// by default.
	Invoke(fn func(), isBlock ...bool) error
	// AfterFunc schedules f to run after d, matching the semantics of [time.AfterFunc].
	//
	// Inside a global handler, it calls proxy.AfterFunc; inside an actor processor, it calls
	// actor.AfterFunc. The returned [Timer] can be cancelled with Timer.Stop.
	AfterFunc(d time.Duration, f func()) (*Timer, error)
	// AfterInvoke schedules f to run after d in a thread-safe manner.
	//
	// After the delay, f is executed serially through the task queue, which keeps it thread-safe.
	// Inside a global handler, it calls proxy.AfterInvoke; inside an actor processor, it calls
	// actor.AfterInvoke. The returned [Timer] can be cancelled with Timer.Stop.
	AfterInvoke(d time.Duration, f func()) (*Timer, error)
	// NewMeshClient creates a new mesh service client.
	//
	// The target can be given in three forms:
	//   - direct connection: direct://127.0.0.1:8011
	//   - direct connection: direct://711baf8d-8a06-11ef-b7df-f4f19e1f0070
	//   - service discovery: discovery://service_name
	NewMeshClient(target string) (transport.Client, error)
	// storeActor stores the current actor.
	storeActor(actor *Actor)
	// deleteActor deletes the current actor.
	deleteActor()
	// incrVersion increments the version and returns the new value.
	incrVersion() int32
	// decrVersion decrements the version and returns the new value.
	decrVersion() int32
	// loadVersion returns the current version.
	loadVersion() int32
	// compareVersionRecycle recycles the object after comparing the version.
	//
	// When the versions match, it runs the post route handlers and releases the context object.
	compareVersionRecycle(version int32)
	// compareVersionExecDefer executes the defer call stack.
	//
	// When the versions match, it fires the top function of the stack.
	compareVersionExecDefer(version int32)
	// cancelDefer cancels the defer call stack.
	cancelDefer()
	// recoverDefer recovers the defer call stack.
	recoverDefer()
	// releaseDefer releases the defer call stack. It clears the fields and returns the context
	// object to the pool.
	releaseDefer()
	// release releases the context. It clears the fields and returns the context object to the
	// pool.
	release()
}

// Kind is the message kind.
type Kind int

// The kinds of a message.
const (
	Event   Kind = iota // Event: an event message
	Request             // Request: a request message
)
