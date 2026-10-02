// Package cluster defines the kinds, states, events, hooks and message arguments shared by the
// gate, node and mesh components.
package cluster

import (
	"github.com/dobyte/due/v2/core/def"
	"github.com/dobyte/due/v2/session"
)

// The kinds of a cluster instance.
const (
	Gate   Kind = iota + 1 // Gate is the gate server
	Node                   // Node is the node server
	Mesh                   // Mesh is the mesh service
	Master                 // Master is the master server
)

// Kind is the type of a cluster instance.
type Kind int

func (k Kind) String() string {
	switch k {
	case Gate:
		return "gate"
	case Node:
		return "node"
	case Mesh:
		return "mesh"
	default:
		return "master"
	}
}

// The states of a cluster instance.
const (
	Shut State = iota // Shut means the node is shut down and cannot be accessed
	Work              // Work means the node is healthy and more players may be assigned to it
	Busy              // Busy means the node is low on resources and should not receive more players
	Hang              // Hang means the node is about to be destroyed and is reclaiming resources
)

// State is the state of a cluster instance.
type State int

func (s State) String() string {
	switch s {
	case Work:
		return "work"
	case Busy:
		return "busy"
	case Hang:
		return "hang"
	default:
		return "shut"
	}
}

// The events of a cluster instance.
const (
	Connect    Event = iota + 1 // Connect is emitted when a connection is opened
	Reconnect                   // Reconnect is emitted when a connection reconnects after a break
	Disconnect                  // Disconnect is emitted when a connection is closed
)

// Event is the event of a cluster instance.
type Event int

func (e Event) String() string {
	switch e {
	case Connect:
		return "connect"
	case Reconnect:
		return "reconnect"
	case Disconnect:
		return "disconnect"
	}

	return ""
}

// The lifecycle hooks of a component.
const (
	Init    Hook = iota // Init initializes the component
	Start               // Start starts the component
	Close               // Close closes the component
	Destroy             // Destroy destroys the component
)

// Hook is a lifecycle hook of a component.
type Hook int

func (h Hook) String() string {
	switch h {
	case Start:
		return "start"
	case Close:
		return "close"
	case Destroy:
		return "destroy"
	default:
		return "init"
	}
}

// Dispatch is the dispatch strategy of stateless route messages.
type Dispatch = def.Dispatch

// The dispatch strategies of stateless route messages.
const (
	Random             = def.Random             // Random dispatches messages randomly
	RoundRobin         = def.RoundRobin         // RoundRobin dispatches messages in turn
	WeightedRoundRobin = def.WeightedRoundRobin // WeightedRoundRobin dispatches messages in turn by weight
)

// GetIPArgs is the arguments of getting the client IP.
type GetIPArgs struct {
	GID    string       // GID is the gate ID, it may be omitted when the session kind is user
	Kind   session.Kind // Kind is the session kind, either session.Conn or session.User
	Target int64        // Target is the session target, either a CID or a UID
}

// Message is a cluster message.
type Message struct {
	Seq   int32 // Seq is the sequence number
	Route int32 // Route is the route ID
	Data  any   // Data is the message payload, which accepts json, proto or []byte
}

// PushArgs is the arguments of pushing a message.
type PushArgs struct {
	GID        string       // GID is the gate ID, it may be omitted when the session kind is user
	Kind       session.Kind // Kind is the session kind, either session.Conn or session.User
	Target     int64        // Target is the session target, either a CID or a UID
	Message    *Message     // Message is the message to push
	Disconnect bool         // Disconnect reports whether to gracefully close the connection after pushing
	Ack        bool         // Ack reports whether the push result should be acknowledged
}

// MulticastArgs is the arguments of multicasting a message.
type MulticastArgs struct {
	GID        string       // GID is the gate ID, it may be omitted when the session kind is user
	Kind       session.Kind // Kind is the session kind, either session.Conn or session.User
	Targets    []int64      // Targets are the session targets, either CIDs or UIDs
	Message    *Message     // Message is the message to multicast
	Disconnect bool         // Disconnect reports whether to gracefully close the connection after pushing
	Ack        bool         // Ack reports whether the push result should be acknowledged
}

// BroadcastArgs is the arguments of broadcasting a message.
type BroadcastArgs struct {
	Kind       session.Kind // Kind is the session kind, either session.Conn or session.User
	Message    *Message     // Message is the message to broadcast
	Disconnect bool         // Disconnect reports whether to gracefully close the connection after pushing
	Ack        bool         // Ack reports whether the push result should be acknowledged
}

// SubscribeArgs is the arguments of subscribing to a channel.
type SubscribeArgs struct {
	GID     string       // GID is the gate ID, it may be omitted when the session kind is user
	Kind    session.Kind // Kind is the session kind, either session.Conn or session.User
	Targets []int64      // Targets are the session targets, either CIDs or UIDs
	Channel string       // Channel is the channel name
}

// UnsubscribeArgs is the arguments of unsubscribing from a channel.
type UnsubscribeArgs struct {
	GID     string       // GID is the gate ID, it may be omitted when the session kind is user
	Kind    session.Kind // Kind is the session kind, either session.Conn or session.User
	Targets []int64      // Targets are the session targets, either CIDs or UIDs
	Channel string       // Channel is the channel name
}

// PublishArgs is the arguments of publishing a message to a channel.
type PublishArgs struct {
	Channel    string   // Channel is the channel name
	Message    *Message // Message is the message to publish
	Disconnect bool     // Disconnect reports whether to gracefully close the connection after pushing
	Ack        bool     // Ack reports whether the push result should be acknowledged
}

// TriggerArgs is the arguments of triggering an event.
type TriggerArgs struct {
	Event int   // Event is the event type
	CID   int64 // CID is the connection ID
	UID   int64 // UID is the user ID
}

// IsOnlineArgs is the arguments of checking whether a session is online.
type IsOnlineArgs struct {
	GID    string       // GID is the gate ID, it may be omitted when the session kind is user
	Kind   session.Kind // Kind is the session kind, either session.Conn or session.User
	Target int64        // Target is the session target, either a CID or a UID
}

// DisconnectArgs is the arguments of disconnecting a session.
type DisconnectArgs struct {
	GID    string       // GID is the gate ID, it may be omitted when the session kind is user
	Kind   session.Kind // Kind is the session kind, either session.Conn or session.User
	Target int64        // Target is the session target, either a CID or a UID
	Force  bool         // Force reports whether to force the disconnection
}

// DeliverArgs is the arguments of delivering a message.
type DeliverArgs struct {
	NID     string   // NID is the receiving node. When it is set, the message is delivered to that node directly; otherwise the system locates the node the user belongs to and delivers the message there
	UID     int64    // UID is the user ID
	Message *Message // Message is the message to deliver
}
