package netpoll

import (
	"github.com/cloudwego/netpoll"
	"github.com/symsimmy/due/errors"
	"reflect"
	"sync"
	"sync/atomic"
)

type connMgr struct {
	id     int64         // 连接ID
	pool   sync.Pool     // 连接池
	conns  [100]sync.Map // 连接集合
	total  int64         // 总连接数
	server *server       // 服务器
}

func newConnMgr(server *server) *connMgr {
	return &connMgr{
		server: server,
		pool:   sync.Pool{New: func() interface{} { return &serverConn{} }},
	}
}

// 关闭连接
func (cm *connMgr) close() {
	for i := range cm.conns {
		cm.conns[i].Range(func(_, conn any) bool {
			_ = conn.(*serverConn).Close(false)
			atomic.AddInt64(&cm.total, -1)
			return true
		})
	}
}

// 分配连接
func (cm *connMgr) allocate(c netpoll.Connection) error {
	if atomic.LoadInt64(&cm.total) >= int64(cm.server.opts.maxConnNum) {
		return errors.ErrTooManyConnection
	}

	id := atomic.AddInt64(&cm.id, 1)
	conn := cm.pool.Get().(*serverConn)
	if err := conn.init(id, c, cm); err != nil {
		cm.pool.Put(conn)
		return err
	}

	index := int(reflect.ValueOf(conn.conn).Pointer()) % len(cm.conns)
	cm.conns[index].Store(c, conn)
	atomic.AddInt64(&cm.total, 1)

	return nil
}

// 回收连接
func (cm *connMgr) recycle(conn *serverConn) {
	index := int(reflect.ValueOf(conn.conn).Pointer()) % len(cm.conns)
	cm.conns[index].Delete(conn.conn)
	cm.pool.Put(conn)
	atomic.AddInt64(&cm.total, -1)
}

// 加载连接
func (cm *connMgr) load(c netpoll.Connection) (*serverConn, bool) {
	index := int(reflect.ValueOf(c).Pointer()) % len(cm.conns)
	v, ok := cm.conns[index].Load(c)
	if !ok {
		return nil, false
	}

	return v.(*serverConn), true
}
