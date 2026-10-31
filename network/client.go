package network

// Client is the client interface.
type Client interface {
	// Dial dials a connection.
	//
	// When addr is empty the address from the client options is used. The connect handler runs on
	// the connection read goroutine before the first frame is read, so it may run concurrently with
	// or after Dial returns.
	Dial(addr ...string) (Conn, error)
	// Protocol returns the protocol name.
	Protocol() string
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
