package server

import (
	"net"
	"sync"
)

type ConnMgr struct {
	server *Server
	rw     sync.RWMutex
	conns  map[net.Conn]*Conn
}

func newConnMgr(s *Server) *ConnMgr {
	return &ConnMgr{
		server: s,
		conns:  make(map[net.Conn]*Conn),
	}
}

func (cm *ConnMgr) allocate(cn net.Conn) error {
	conn := newConn(cm, cn)
	cm.rw.Lock()
	cm.conns[cn] = conn
	cm.rw.Unlock()
	return nil
}

// 关闭连接
func (cm *ConnMgr) close() {
	//var wg sync.WaitGroup
	//
	//wg.Add(len(cm.partitions))
	//
	//for i := range cm.conns {
	//	c := cm.conns[i]
	//
	//	xcall.Go(func() {
	//		c.close()
	//		wg.Done()
	//	})
	//}
	//
	//wg.Wait()
}
