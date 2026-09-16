package quic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
	"github.com/quic-go/quic-go"
)

type client struct {
	opts              *clientOptions
	id                atomic.Int64
	connectHandler    network.ConnectHandler
	disconnectHandler network.DisconnectHandler
	receiveHandler    network.ReceiveHandler
	heartbeatHandler  network.HeartbeatHandler
}

var _ network.Client = (*client)(nil)

// NewClient creates a QUIC client. Register handlers before dialing.
func NewClient(opts ...ClientOption) network.Client {
	o := defaultClientOptions()
	for _, opt := range opts {
		opt(o)
	}
	if o.tlsConfig == nil {
		o.tlsConfig = &tls.Config{}
	} else {
		o.tlsConfig = o.tlsConfig.Clone()
	}
	o.tlsConfig.NextProtos = []string{alpn}
	return &client{opts: o}
}

// Dial establishes one bidirectional stream. A zero dial timeout disables the
// overall deadline; QUIC still applies its transport handshake timeout.
// OnConnect is dispatched before OnReceive, independently of Dial returning.
func (c *client) Dial(addr ...string) (network.Conn, error) {
	if c.opts.tlsErr != nil {
		return nil, c.opts.tlsErr
	}
	address := c.opts.addr
	if len(addr) > 0 && addr[0] != "" {
		address = addr[0]
	}
	ctx := context.Background()
	if c.opts.dialTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.opts.dialTimeout)
		defer cancel()
	}
	config := transportConfig(c.opts.heartbeatInterval)
	config.MaxIncomingStreams = -1
	config.HandshakeIdleTimeout = c.opts.dialTimeout
	// Keep the hostname so quic-go can derive the TLS ServerName.
	qc, err := quic.DialAddr(ctx, address, c.opts.tlsConfig, config)
	if err != nil {
		return nil, err
	}
	stream, err := qc.OpenStreamSync(ctx)
	if err != nil {
		_ = qc.CloseWithError(0, "open stream failed")
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = stream.SetWriteDeadline(deadline)
	}
	// Opening alone does not announce a QUIC stream. A normal heartbeat makes
	// server-initiated messages possible even when periodic heartbeats are off.
	hb := packet.PackHeartbeat()
	err = writeBuffer(stream, hb)
	hb.Release()
	if err != nil {
		_ = qc.CloseWithError(0, "initialize stream failed")
		return nil, err
	}
	_ = stream.SetWriteDeadline(time.Time{})
	return newClientConn(c.id.Add(1), qc, stream, c), nil
}

// Protocol returns the protocol name.
func (c *client) Protocol() string { return protocol }

// OnConnect registers the connection handler.
func (c *client) OnConnect(h network.ConnectHandler) { c.connectHandler = h }

// OnDisconnect registers the disconnection handler.
func (c *client) OnDisconnect(h network.DisconnectHandler) { c.disconnectHandler = h }

// OnReceive registers the message handler, which owns each received buffer.
func (c *client) OnReceive(h network.ReceiveHandler) { c.receiveHandler = h }

// OnHeartbeat registers the heartbeat handler.
func (c *client) OnHeartbeat(h network.HeartbeatHandler) { c.heartbeatHandler = h }

func makeClientTLSConfig(caFile, serverName string) (*tls.Config, error) {
	config := &tls.Config{ServerName: serverName}
	if caFile == "" {
		return config, nil
	}
	certs, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	config.RootCAs = x509.NewCertPool()
	if !config.RootCAs.AppendCertsFromPEM(certs) {
		return nil, errors.ErrInvalidCertFile
	}
	return config, nil
}

func transportConfig(heartbeat time.Duration) *quic.Config {
	config := &quic.Config{MaxIncomingStreams: 1, MaxIncomingUniStreams: -1}
	if heartbeat > 0 {
		config.MaxIdleTimeout = 3 * heartbeat
		config.KeepAlivePeriod = heartbeat / 2
	}
	// With application heartbeats disabled, keep quic-go's default idle timeout
	// and send transport keepalives so an otherwise healthy idle session survives.
	if heartbeat == 0 {
		config.KeepAlivePeriod = 10 * time.Second
	}
	return config
}
