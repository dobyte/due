/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/28 3:48 下午
 * @Desc: 连接管理器
 */

package ws

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	taskpool "github.com/dobyte/due/v2/task"
	"github.com/gorilla/websocket"
)

type serverConnMgr struct {
	id         atomic.Int64 // 连接ID
	total      atomic.Int64 // 总连接数
	server     *server      // 服务器
	connPool   sync.Pool    // 连接池
	taskPool   sync.Pool    // 任务池
	partitions []*partition // 连接管理
}

func newConnMgr(server *server) *serverConnMgr {
	cm := &serverConnMgr{}
	cm.server = server
	cm.connPool = sync.Pool{New: func() any { return &serverConn{attr: &attr{}, connMgr: cm} }}
	cm.taskPool = sync.Pool{New: func() any { return &task{} }}
	cm.partitions = make([]*partition, runtime.NumCPU()*2)

	for i := 0; i < len(cm.partitions); i++ {
		cm.partitions[i] = &partition{connections: make(map[*websocket.Conn]*serverConn)}
	}

	return cm
}

// 关闭连接
func (cm *serverConnMgr) close() {
	wg, _ := taskpool.WithContext(context.Background())

	for _, p := range cm.partitions {
		wg.Go(p.close)
	}

	wg.Wait()
}

// 分配连接
func (cm *serverConnMgr) allocateConn(c *websocket.Conn) error {
	maxConnNum := int64(cm.server.opts.maxConnNum)
	for {
		if total := cm.total.Load(); total >= maxConnNum {
			return errors.ErrTooManyConnection
		} else if cm.total.CompareAndSwap(total, total+1) {
			break
		}
	}

	conn := cm.connPool.Get().(*serverConn)
	index := int(uintptr(unsafe.Pointer(c))) % len(cm.partitions)
	cm.partitions[index].store(c, conn)
	conn.init(c)

	return nil
}

// 回收连接
func (cm *serverConnMgr) recycleConn(c *websocket.Conn) {
	index := int(uintptr(unsafe.Pointer(c))) % len(cm.partitions)
	if conn, ok := cm.partitions[index].delete(c); ok {
		conn.reset()
		cm.connPool.Put(conn)
		cm.total.Add(-1)
	}
}

// 分配任务对象
func (cm *serverConnMgr) allocateTask(typ int8, msg ...[]byte) *task {
	t := cm.taskPool.Get().(*task)
	t.typ = typ
	if len(msg) > 0 {
		t.msg = msg[0]
	}

	return t
}

// 回收任务到对象池
func (cm *serverConnMgr) recycleTask(t *task) {
	t.msg = nil
	cm.taskPool.Put(t)
}

type partition struct {
	rw          sync.RWMutex
	connections map[*websocket.Conn]*serverConn
}

// 存储连接
func (p *partition) store(c *websocket.Conn, conn *serverConn) {
	p.rw.Lock()
	p.connections[c] = conn
	p.rw.Unlock()
}

// 删除连接
func (p *partition) delete(c *websocket.Conn) (*serverConn, bool) {
	p.rw.Lock()
	conn, ok := p.connections[c]
	if ok {
		delete(p.connections, c)
	}
	p.rw.Unlock()

	return conn, ok
}

// 关闭该分片内的所有连接
func (p *partition) close() error {
	p.rw.RLock()
	conns := make([]network.Conn, 0, len(p.connections))
	for _, conn := range p.connections {
		conns = append(conns, conn)
	}
	p.rw.RUnlock()

	wg, _ := taskpool.WithContext(context.Background())

	for _, conn := range conns {
		wg.Go(func() error {
			return conn.Close()
		})
	}

	return wg.Wait()
}
