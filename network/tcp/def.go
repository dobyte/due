package tcp

import (
	"crypto/tls"
	"net"

	"github.com/pires/go-proxyproto"
)

// protocol is the protocol identifier.
const protocol = "tcp"

// maxBatchWriteNum is the maximum number of tasks in a single batch write.
const maxBatchWriteNum = 64

// minWriteQueueSize is the minimum write queue size.
const minWriteQueueSize = 128

// setNoDelay enables the TCP_NODELAY option, which turns off the Nagle algorithm.
func setNoDelay(conn net.Conn) {
	switch ccc := conn.(type) {
	case *proxyproto.Conn:
		switch cc := ccc.Raw().(type) {
		case *net.TCPConn:
			cc.SetNoDelay(true)
		case *tls.Conn:
			if c, ok := cc.NetConn().(*net.TCPConn); ok {
				c.SetNoDelay(true)
			}
		}
	case *tls.Conn:
		if c, ok := ccc.NetConn().(*net.TCPConn); ok {
			c.SetNoDelay(true)
		}
	default:
		if c, ok := conn.(*net.TCPConn); ok {
			c.SetNoDelay(true)
		}
	}
}
