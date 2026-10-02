package tcp

import (
	"crypto/tls"
	"net"
	"sync/atomic"

	"github.com/dobyte/due/v2/network"
)

type client struct {
	opts              *clientOptions            // Options
	cid               atomic.Int64              // Connection ID
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

	return c
}

// Dial dials a connection.
//
// When addr is empty the address from the client options is used.
func (c *client) Dial(addr ...string) (network.Conn, error) {
	var address string

	if len(addr) > 0 && addr[0] != "" {
		address = addr[0]
	} else {
		address = c.opts.addr
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	var conn net.Conn

	if c.opts.tlsConfig != nil {
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: c.opts.dialTimeout}, tcpAddr.Network(), tcpAddr.String(), c.opts.tlsConfig)
	} else {
		conn, err = net.DialTimeout(tcpAddr.Network(), tcpAddr.String(), c.opts.dialTimeout)
	}

	if err != nil {
		return nil, err
	}

	setNoDelay(conn)

	return newClientConn(c, conn), nil
}

// Protocol returns the protocol name.
func (c *client) Protocol() string {
	return protocol
}

// OnConnect registers the handler invoked when a connection is opened.
//
// It must be registered before Dial; registering it after Dial causes a data race.
func (c *client) OnConnect(handler network.ConnectHandler) {
	c.connectHandler = handler
}

// OnDisconnect registers the handler invoked when a connection is closed.
//
// It must be registered before Dial; registering it after Dial causes a data race.
func (c *client) OnDisconnect(handler network.DisconnectHandler) {
	c.disconnectHandler = handler
}

// OnHeartbeat registers the handler invoked when a heartbeat is received.
//
// It must be registered before Dial; registering it after Dial causes a data race.
func (c *client) OnHeartbeat(handler network.HeartbeatHandler) {
	c.heartbeatHandler = handler
}

// OnReceive registers the handler invoked when a message is received.
//
// It must be registered before Dial; registering it after Dial causes a data race.
func (c *client) OnReceive(handler network.ReceiveHandler) {
	c.receiveHandler = handler
}
