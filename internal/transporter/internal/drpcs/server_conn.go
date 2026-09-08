package drpcs

import "net"

type ServerConn struct {
	svr  *Server
	conn *net.TCPConn
}

func newServerConn(svr *Server, conn *net.TCPConn) *ServerConn {

}
