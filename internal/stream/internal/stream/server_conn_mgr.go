package stream

import (
	"net"
	"sync"
)

type connMgr struct {
	server *Server
	rw     sync.RWMutex
	conns  map[net.Conn]*ServerConn
}

func newConnMgr(s *Server) *connMgr {
	return &connMgr{
		server: s,
		conns:  make(map[net.Conn]*ServerConn),
	}
}

func (cm *connMgr) allocate(cn net.Conn) error {
	conn := newConn(cm, cn)
	cm.rw.Lock()
	cm.conns[cn] = conn
	cm.rw.Unlock()
	return nil
}

// 关闭连接
func (cm *connMgr) close() {
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
