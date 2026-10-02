package network

import (
	"net"

	"github.com/dobyte/due/v2/core/buffer"
)

const (
	ConnOpened ConnState = iota + 1 // Connection opened
	ConnHanged                      // Connection hanged
	ConnClosed                      // Connection closed
)

type (
	// ConnState is the connection state.
	ConnState int32

	// Conn is the connection interface.
	Conn interface {
		// ID returns the connection ID.
		ID() int64
		// UID returns the user ID.
		UID() int64
		// Attr returns the attribute interface of the connection.
		Attr() Attr
		// Bind binds the connection to the given user ID.
		Bind(uid int64) error
		// Unbind unbinds the user ID from the connection.
		Unbind() error
		// Push sends a message with low priority.
		//
		// The caller controls when buf is released when the send fails.
		Push(buf buffer.Buffer) error
		// State returns the connection state.
		State() ConnState
		// Close closes the connection.
		//
		// When force is true the connection is closed forcibly.
		Close(force ...bool) error
		// LocalIP returns the local IP address.
		LocalIP() (string, error)
		// LocalAddr returns the local address.
		LocalAddr() (net.Addr, error)
		// RemoteIP returns the remote IP address.
		RemoteIP() (string, error)
		// RemoteAddr returns the remote address.
		RemoteAddr() (net.Addr, error)
	}

	// Attr is the attribute interface of a connection.
	Attr interface {
		// Set sets the value of the attribute with the given key.
		Set(key, value any)
		// Get returns the value of the attribute with the given key and whether it exists.
		Get(key any) (any, bool)
		// Del deletes the attribute with the given key and reports whether the deletion succeeded.
		Del(key any) bool
		// Clear removes all attributes.
		Clear()
		// Visit calls fn for every attribute and stops when fn returns false.
		Visit(fn func(key, value any) bool)
	}
)
