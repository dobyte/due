package kcp

import (
	"github.com/symsimmy/due/network"
	"net"
	"sync"
)

type serverConnMgr struct {
	rw     sync.RWMutex             // 连接读写锁
	id     int64                    // 连接ID
	pool   sync.Pool                // 连接池
	conns  map[net.Conn]*serverConn // 连接集合
	server *server                  // 服务器
}

func newConnMgr(server *server) *serverConnMgr {
	return &serverConnMgr{
		server: server,
		conns:  make(map[net.Conn]*serverConn),
		pool:   sync.Pool{New: func() interface{} { return &serverConn{} }},
	}
}

// 关闭连接
func (cm *serverConnMgr) close() {
	cm.rw.Lock()
	defer cm.rw.RUnlock()

	for _, conn := range cm.conns {
		_ = conn.Close()
	}
}

// 分配连接
func (cm *serverConnMgr) allocate(c net.Conn) error {
	cm.rw.Lock()
	defer cm.rw.Unlock()

	if len(cm.conns) >= cm.server.opts.maxConnNum {
		return network.ErrTooManyConnection
	}

	cm.id++
	conn := cm.pool.Get().(*serverConn)
	conn.init(c, cm)
	cm.conns[c] = conn

	return nil
}

// 回收连接
func (cm *serverConnMgr) recycle(conn *serverConn) {
	cm.rw.Lock()
	defer cm.rw.Unlock()

	delete(cm.conns, conn.conn)
	cm.pool.Put(conn)
}
