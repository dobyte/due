package kcp

import (
	"sync/atomic"

	"github.com/dobyte/due/v2/network"
	"github.com/xtaci/kcp-go/v5"
)

type client struct {
	opts              *clientOptions            // Options
	cid               atomic.Int64              // Connection ID
	connectHandler    network.ConnectHandler    // Hook invoked when a connection is opened
	disconnectHandler network.DisconnectHandler // Hook invoked when a connection is closed
	heartbeatHandler  network.HeartbeatHandler  // Hook invoked on connection heartbeat
	receiveHandler    network.ReceiveHandler    // Hook invoked when a message is received
	handshakeHandler  network.HandshakeHandler  // Hook invoked when a handshake frame is received
}

var _ network.Client = &client{}

// NewClient returns a new KCP client.
//
// The default options are overridden by the given opts before the underlying client is initialized.
func NewClient(opts ...ClientOption) network.Client {
	o := defaultClientOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &client{}
	c.opts = o

	return c
}

// Dial dials the server and returns the resulting connection.
//
// When no address is given, the address configured on the client is used. The underlying socket is
// an unconnected UDP socket owned by the session, so closing the connection also closes the socket;
// a connected socket would make the internal WriteTo calls of kcp-go fail so that no data could
// ever be sent.
func (c *client) Dial(addr ...string) (network.Conn, error) {
	var address string
	if len(addr) > 0 && addr[0] != "" {
		address = addr[0]
	} else {
		address = c.opts.addr
	}

	conn, err := kcp.DialWithOptions(address, nil, 0, 0)
	if err != nil {
		return nil, err
	}

	return newClientConn(c, conn), nil
}

// Protocol returns the protocol name "kcp".
func (c *client) Protocol() string {
	return protocol
}

// OnConnect registers handler to be invoked when a connection is opened.
func (c *client) OnConnect(handler network.ConnectHandler) {
	c.connectHandler = handler
}

// OnDisconnect registers handler to be invoked when a connection is closed.
func (c *client) OnDisconnect(handler network.DisconnectHandler) {
	c.disconnectHandler = handler
}

// OnHeartbeat registers handler to be invoked on a connection heartbeat.
func (c *client) OnHeartbeat(handler network.HeartbeatHandler) {
	c.heartbeatHandler = handler
}

// OnReceive registers handler to be invoked when a message is received.
func (c *client) OnReceive(handler network.ReceiveHandler) {
	c.receiveHandler = handler
}

// OnHandshake registers handler to be invoked when a handshake frame is received.
func (c *client) OnHandshake(handler network.HandshakeHandler) {
	c.handshakeHandler = handler
}
