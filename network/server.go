package network

import "github.com/dobyte/due/v2/core/buffer"

type (
	// StartHandler is the handler invoked when the server starts.
	StartHandler func()
	// CloseHandler is the handler invoked when the server stops.
	CloseHandler func()
	// ConnectHandler is the handler invoked when a connection is opened.
	//
	// The connect, heartbeat, receive and handshake handlers all run on the connection read
	// goroutine, so closing the connection or stopping the server synchronously from any of them
	// would deadlock; perform those operations from another goroutine. The connect handler runs
	// before the first frame is read and, on the client side, may run concurrently with or after
	// Dial returns.
	ConnectHandler func(conn Conn)
	// DisconnectHandler is the handler invoked when a connection is closed.
	//
	// It runs after the connection read goroutine has exited and after the connect handler has
	// returned, so it never runs concurrently with the receive or connect handler of the same
	// connection. Releasing per-connection state (sessions, buffers, rate limiter buckets) here is
	// therefore safe. On the client side it may run before Dial returns.
	DisconnectHandler func(conn Conn)
	// HeartbeatHandler is the handler invoked when a heartbeat is received.
	//
	// It runs on the connection read goroutine; see ConnectHandler for the synchronous-close
	// caveat. heartbeatTime is the time in nanoseconds at which the client received the heartbeat
	// sent by the server.
	HeartbeatHandler func(conn Conn, heartbeatTime int64)
	// ReceiveHandler is the handler invoked when a message is received.
	//
	// It runs on the connection read goroutine; see ConnectHandler for the synchronous-close
	// caveat. The user layer controls when buf is released to avoid a memory leak.
	ReceiveHandler func(conn Conn, buf buffer.Buffer)
	// HandshakeHandler is the handler invoked when a handshake frame is received.
	//
	// Handshake frames are marked by a reserved bit of the packet header instead of a route, and
	// they are dispatched before the receive handler. It runs on the connection read goroutine; see
	// ConnectHandler for the synchronous-close caveat. The user layer controls when buf is
	// released to avoid a memory leak.
	HandshakeHandler func(conn Conn, buf buffer.Buffer)
)

// Server is the server interface.
type Server interface {
	// Addr returns the listen address.
	Addr() string
	// Start starts the server.
	Start() error
	// Stop stops the server.
	Stop() error
	// Protocol returns the protocol name.
	Protocol() string
	// OnStart registers the handler invoked when the server starts.
	OnStart(handler StartHandler)
	// OnStop registers the handler invoked when the server stops.
	OnStop(handler CloseHandler)
	// OnConnect registers the handler invoked when a connection is opened.
	OnConnect(handler ConnectHandler)
	// OnHeartbeat registers the handler invoked when a heartbeat is received.
	OnHeartbeat(handler HeartbeatHandler)
	// OnReceive registers the handler invoked when a message is received.
	OnReceive(handler ReceiveHandler)
	// OnHandshake registers the handler invoked when a handshake frame is received.
	OnHandshake(handler HandshakeHandler)
	// OnDisconnect registers the handler invoked when a connection is closed.
	OnDisconnect(handler DisconnectHandler)
}
