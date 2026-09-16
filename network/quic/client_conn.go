package quic

import "github.com/quic-go/quic-go"

func newClientConn(id int64, qc *quic.Conn, stream *quic.Stream, client *client) *conn {
	c := newConn(id, qc, stream, connOptions{
		queueSize:         client.opts.writeQueueSize,
		writeTimeout:      client.opts.writeTimeout,
		closeTimeout:      client.opts.closeTimeout,
		heartbeatInterval: client.opts.heartbeatInterval,
		connect:           client.connectHandler,
		disconnect:        client.disconnectHandler,
		receive:           client.receiveHandler,
		heartbeat:         client.heartbeatHandler,
	})
	c.start()
	return c
}
