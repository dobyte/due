/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/28 3:48 下午
 * @Desc: 连接管理器
 */

package ws

import (
	"github.com/dobyte/due/v2/network"
	"github.com/gorilla/websocket"
	"sync"
)

type connMgr struct {
	mu     sync.Mutex            // 连接读写锁
	id     int64                 // 连接ID
	pool   sync.Pool             // 连接池
	conns  map[int64]*serverConn // 连接集合
	server *server               // 服务器
}

func NewConnMgr() *connMgr {
	return newConnMgr(nil)
}

func newConnMgr(server *server) *connMgr {
	return &connMgr{
		server: server,
		conns:  make(map[int64]*serverConn),
		pool:   sync.Pool{New: func() interface{} { return &serverConn{} }},
	}
}

// 关闭连接
func (cm *connMgr) close() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, conn := range cm.conns {
		_ = conn.graceClose(false)
	}

	cm.conns = nil
}

// 分配连接
func (cm *connMgr) allocate(c *websocket.Conn) error {
	cm.mu.Lock()

	if len(cm.conns) >= cm.server.opts.maxConnNum {
		cm.mu.Unlock()
		return network.ErrTooManyConnection
	}

	cm.id++
	id := cm.id
	conn := cm.pool.Get().(*serverConn)
	cm.conns[id] = conn
	cm.mu.Unlock()

	conn.init(id, c, cm)

	return nil
}

// 回收连接
func (cm *connMgr) recycle(conn *serverConn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	delete(cm.conns, conn.id)
	cm.pool.Put(conn)
}
