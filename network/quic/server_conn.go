package quic

import "github.com/quic-go/quic-go"

func newServerConn(id int64, qc *quic.Conn, stream *quic.Stream, server *server) *conn {
	return newConn(id, qc, stream, connOptions{
		server:             true,
		queueSize:          server.opts.writeQueueSize,
		writeTimeout:       server.opts.writeTimeout,
		closeTimeout:       server.opts.closeTimeout,
		heartbeatInterval:  server.opts.heartbeatInterval,
		heartbeatMechanism: server.opts.heartbeatMechanism,
		authorizeTimeout:   server.opts.authorizeTimeout,
		connect:            server.connectHandler,
		disconnect:         server.disconnectHandler,
		receive:            server.receiveHandler,
		heartbeat:          server.heartbeatHandler,
	})
}
