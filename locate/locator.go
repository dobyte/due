// Package locate defines the locator abstraction used to track the gate and
// node a user is bound to.
package locate

import (
	"context"
)

// Locator locates the gate and the node a user is bound to.
type Locator interface {
	// Name returns the component name of the locator.
	Name() string
	// Watch watches changes to user locations, optionally filtered by kinds.
	Watch(ctx context.Context, kinds ...string) (Watcher, error)
	// BindGate binds the user to the gate identified by gid.
	BindGate(ctx context.Context, uid int64, gid string) error
	// BindNode binds the user to the node identified by name and nid.
	BindNode(ctx context.Context, uid int64, name, nid string) error
	// UnbindGate unbinds the user from the gate identified by gid.
	UnbindGate(ctx context.Context, uid int64, gid string) error
	// UnbindNode unbinds the user from the node identified by name and nid.
	UnbindNode(ctx context.Context, uid int64, name string, nid string) error
	// LocateGate locates the gate the user is bound to.
	LocateGate(ctx context.Context, uid int64) (string, error)
	// LocateNode locates the node the user is bound to under the given name.
	LocateNode(ctx context.Context, uid int64, name string) (string, error)
	// LocateNodes locates every node the user is bound to.
	LocateNodes(ctx context.Context, uid int64) (map[string]string, error)
	// Close closes the locator.
	Close() error
}

// Watcher watches changes to user locations.
type Watcher interface {
	// Next returns the next batch of user location events.
	Next() ([]*Event, error)
	// Stop stops watching.
	Stop() error
}

// Event describes a change to a user location.
type Event struct {
	UID     int64     `json:"uid"`     // User ID
	Type    EventType `json:"type"`    // Event type
	InsID   string    `json:"insID"`   // Instance ID
	InsKind string    `json:"insKind"` // Instance kind
	InsName string    `json:"insName"` // Instance name
}

// EventType is the type of a location event.
type EventType int

// The types of a location event.
const (
	BindGate   EventType = iota + 1 // BindGate: the user is bound to a gate
	BindNode                        // BindNode: the user is bound to a node
	UnbindGate                      // UnbindGate: the user is unbound from a gate
	UnbindNode                      // UnbindNode: the user is unbound from a node
)
