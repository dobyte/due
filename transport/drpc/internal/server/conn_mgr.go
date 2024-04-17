package server

import (
	"github.com/dobyte/due/v2/utils/xcall"
	"net"
	"sync"
)

type connMgr struct {
	rw    sync.RWMutex
	conns map[net.Conn]*conn
}

func newConnMgr() *connMgr {
	return &connMgr{
		conns: make(map[net.Conn]*conn),
	}
}

// 关闭连接
func (cm *connMgr) close() {
	var wg sync.WaitGroup

	wg.Add(len(cm.partitions))

	for i := range cm.conns {
		c := cm.conns[i]

		xcall.Go(func() {
			c.close()
			wg.Done()
		})
	}

	wg.Wait()
}
