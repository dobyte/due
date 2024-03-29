/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/15 9:55 下午
 * @Desc: TODO
 */

package tcp

import (
	"github.com/symsimmy/due/network"
	"net"
	"sync"
	"sync/atomic"
)

type serverConnMgr struct {
	id     int64     // 连接ID
	pool   sync.Pool // 连接池
	conns  sync.Map  // 连接集合
	server *server   // 服务器
}

func newConnMgr(server *server) *serverConnMgr {
	return &serverConnMgr{
		server: server,
		pool:   sync.Pool{New: func() interface{} { return &serverConn{} }},
	}
}

// 关闭连接
func (cm *serverConnMgr) close() {
	cm.conns.Range(func(k, v interface{}) bool {
		conn := v.(network.Conn)
		_ = conn.Close(false)
		return true
	})
}

// 分配连接
func (cm *serverConnMgr) allocate(c net.Conn) error {
	atomic.AddInt64(&cm.id, 1)
	conn := cm.pool.Get().(*serverConn)
	cm.conns.Store(c, conn)

	conn.init(c, cm)

	return nil
}

// 回收连接
func (cm *serverConnMgr) recycle(conn *serverConn) {
	cm.conns.Delete(conn.conn)
	conn.conn = nil
	cm.pool.Put(conn)

}
