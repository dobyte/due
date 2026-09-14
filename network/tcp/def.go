package tcp

import (
	"crypto/tls"
	"net"

	"github.com/pires/go-proxyproto"
)

// protocol 协议标识
const protocol = "tcp"

// maxBatchWriteNum 单次批量写入的最大任务数
const maxBatchWriteNum = 64

// setNoDelay 设置Nagle算法
// @param conn net.Conn TCP连接
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
