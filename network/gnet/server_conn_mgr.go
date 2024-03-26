/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/15 9:55 下午
 * @Desc: TODO
 */

package gnet

import (
	"github.com/dobyte/due/v2/errors"
	"github.com/panjf2000/gnet/v2"
	"sync"
	"sync/atomic"
)

type serverConnMgr struct {
	id          int64                     // 连接ID
	pool        sync.Pool                 // 连接池
	total       int64                     // 连接总数
	rw          sync.RWMutex              // 连接锁
	conns       map[gnet.Conn]*serverConn // 连接集合
	server      *server                   // 服务器
	chHeartbeat chan *serverConn          // 心跳检测
}

func newConnMgr(server *server) *serverConnMgr {
	cm := &serverConnMgr{}
	cm.server = server
	cm.conns = make(map[gnet.Conn]*serverConn)
	cm.pool = sync.Pool{New: func() interface{} { return &serverConn{} }}
	cm.chHeartbeat = make(chan *serverConn, 200)

	cm.init()

	return cm
}

// 创建携程池处理心跳检测
func (cm *serverConnMgr) init() {
	for i := 0; i < 200; i++ {
		go func() {
			for {
				select {
				case conn, ok := <-cm.chHeartbeat:
					if !ok {
						return
					}

					conn.checkHeartbeat()
				}
			}
		}()
	}
}

// 关闭连接
func (cm *serverConnMgr) close() {
	cm.rw.Lock()
	defer cm.rw.Unlock()

	for _, conn := range cm.conns {
		_ = conn.graceClose(false)
	}

	cm.conns = nil
	close(cm.chHeartbeat)
}

// 分配连接
func (cm *serverConnMgr) allocate(c gnet.Conn) error {
	if atomic.LoadInt64(&cm.total) >= int64(cm.server.opts.maxConnNum) {
		return errors.ErrTooManyConnection
	}

	id := atomic.AddInt64(&cm.id, 1)
	conn := cm.pool.Get().(*serverConn)
	conn.init(id, c, cm)

	cm.rw.Lock()
	cm.conns[c] = conn
	cm.rw.Unlock()

	return nil
}

// 回收连接
func (cm *serverConnMgr) recycle(conn *serverConn) {
	cm.rw.Lock()
	delete(cm.conns, conn.conn)
	cm.rw.Unlock()

	cm.pool.Put(conn)
}

// 销毁连接
func (cm *serverConnMgr) destroy(c gnet.Conn) {
	cm.rw.Lock()
	conn, ok := cm.conns[c]
	if ok {
		delete(cm.conns, c)
	}
	cm.rw.Unlock()

	if ok {
		cm.pool.Put(conn)
	}
}

// 加载连接
func (cm *serverConnMgr) load(c gnet.Conn) (conn *serverConn, ok bool) {
	cm.rw.RLock()
	conn, ok = cm.conns[c]
	cm.rw.RUnlock()

	return
}

// 检测心跳
func (cm *serverConnMgr) checkHeartbeat() {
	cm.rw.RLock()
	for i := range cm.conns {
		cm.chHeartbeat <- cm.conns[i]
	}
	cm.rw.RUnlock()
}
