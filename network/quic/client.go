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
	cid               atomic.Int64
	connectHandler    network.ConnectHandler
	disconnectHandler network.DisconnectHandler
	receiveHandler    network.ReceiveHandler
	heartbeatHandler  network.HeartbeatHandler
}

var _ network.Client = (*client)(nil)

// NewClient returns a new QUIC client.
//
// Every hook must be registered before dialing.
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

// Dial dials the server and returns the resulting connection.
//
// It establishes a bidirectional stream. When the dial timeout is 0 no overall deadline is set, and
// the handshake timeout of the QUIC transport layer still applies. The connect hook fires before
// the receive hook, regardless of whether Dial has already returned.
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
	// Keep the host name so that quic-go can derive the TLS ServerName.
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
	// Merely opening a stream does not announce it to the peer. After one ordinary heartbeat is
	// sent, the server can push messages to the client even when periodic heartbeats are disabled.
	hb := packet.PackHeartbeat()
	err = writeBuffer(stream, hb)
	hb.Release()
	if err != nil {
		_ = qc.CloseWithError(0, "initialize stream failed")
		return nil, err
	}
	_ = stream.SetWriteDeadline(time.Time{})
	return newClientConn(c, qc, stream), nil
}

// Protocol returns the protocol name.
func (c *client) Protocol() string { return protocol }

// OnConnect registers h to be invoked when a connection is opened.
//
// It must be registered before Dial; registering it afterwards races with Dial.
func (c *client) OnConnect(h network.ConnectHandler) { c.connectHandler = h }

// OnDisconnect registers h to be invoked when a connection is closed.
//
// It must be registered before Dial; registering it afterwards races with Dial.
func (c *client) OnDisconnect(h network.DisconnectHandler) { c.disconnectHandler = h }

// OnReceive registers h to be invoked when a message is received.
//
// It must be registered before Dial; registering it afterwards races with Dial. The handler owns
// every received buffer.
func (c *client) OnReceive(h network.ReceiveHandler) { c.receiveHandler = h }

// OnHeartbeat registers h to be invoked on a connection heartbeat.
//
// It must be registered before Dial; registering it afterwards races with Dial.
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
	// When application-level heartbeats are disabled, keep the default quic-go idle timeout and
	// send transport-level keepalive packets so that healthy but idle sessions stay alive.
	if heartbeat == 0 {
		config.KeepAlivePeriod = 10 * time.Second
	}
	return config
}
