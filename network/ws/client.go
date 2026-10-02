package ws

import (
	"sync/atomic"

	"github.com/dobyte/due/v2/network"
	"github.com/gorilla/websocket"
)

type client struct {
	opts              *clientOptions            // Options
	cid               atomic.Int64              // Connection ID
	dialer            *websocket.Dialer         // Dialer
	connectHandler    network.ConnectHandler    // Handler invoked when a connection is opened
	disconnectHandler network.DisconnectHandler // Handler invoked when a connection is closed
	heartbeatHandler  network.HeartbeatHandler  // Handler invoked when a heartbeat is received
	receiveHandler    network.ReceiveHandler    // Handler invoked when a message is received
}

var _ network.Client = &client{}

// NewClient returns a new client.
func NewClient(opts ...ClientOption) network.Client {
	o := defaultClientOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &client{}
	c.opts = o
	c.dialer = &websocket.Dialer{
		HandshakeTimeout:  o.dialTimeout,
		ReadBufferSize:    o.readBufferSize,
		TLSClientConfig:   o.tlsConfig,
		EnableCompression: o.enableCompression,
	}

	return c
}

// Dial dials a connection.
//
// When addr is empty the URL from the client options is used.
func (c *client) Dial(addr ...string) (network.Conn, error) {
	var url string

	if len(addr) > 0 && addr[0] != "" {
		url = addr[0]
	} else {
		url = c.opts.url
	}

	conn, _, err := c.dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	if c.opts.enableCompression {
		conn.EnableWriteCompression(true)
		conn.SetCompressionLevel(c.opts.compressionLevel)
	}

	return newClientConn(c, conn), nil
}

// Protocol returns the protocol name.
func (c *client) Protocol() string {
	return protocol
}

// OnConnect registers the handler invoked when a connection is opened.
func (c *client) OnConnect(handler network.ConnectHandler) {
	c.connectHandler = handler
}

// OnDisconnect registers the handler invoked when a connection is closed.
func (c *client) OnDisconnect(handler network.DisconnectHandler) {
	c.disconnectHandler = handler
}

// OnHeartbeat registers the handler invoked when a heartbeat is received.
func (c *client) OnHeartbeat(handler network.HeartbeatHandler) {
	c.heartbeatHandler = handler
}

// OnReceive registers the handler invoked when a message is received.
func (c *client) OnReceive(handler network.ReceiveHandler) {
	c.receiveHandler = handler
}
