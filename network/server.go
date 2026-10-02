package network

import "github.com/dobyte/due/v2/core/buffer"

type (
	// StartHandler is the handler invoked when the server starts.
	StartHandler func()
	// CloseHandler is the handler invoked when the server stops.
	CloseHandler func()
	// ConnectHandler is the handler invoked when a connection is opened.
	ConnectHandler func(conn Conn)
	// DisconnectHandler is the handler invoked when a connection is closed.
	DisconnectHandler func(conn Conn)
	// HeartbeatHandler is the handler invoked when a heartbeat is received.
	//
	// heartbeatTime is the time in nanoseconds at which the client received the heartbeat sent by the server.
	HeartbeatHandler func(conn Conn, heartbeatTime int64)
	// ReceiveHandler is the handler invoked when a message is received.
	//
	// The user layer controls when buf is released to avoid a memory leak.
	ReceiveHandler func(conn Conn, buf buffer.Buffer)
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
	// OnDisconnect registers the handler invoked when a connection is closed.
	OnDisconnect(handler DisconnectHandler)
}
