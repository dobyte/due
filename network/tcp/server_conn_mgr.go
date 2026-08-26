package tcp

import (
	"context"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	taskpool "github.com/dobyte/due/v2/task"
)

type serverConnMgr struct {
	id         atomic.Int64 // 连接ID
	total      atomic.Int64 // 总连接数
	server     *server      // 服务器
	connPool   sync.Pool    // 连接池
	taskPool   sync.Pool    // 任务池
	partitions []*partition // 连接管理
}

// newServerConnMgr 创建一个连接管理器
// @param server *server 服务器
// @return @1 *serverConnMgr 连接管理器
func newServerConnMgr(server *server) *serverConnMgr {
	cm := &serverConnMgr{}
	cm.server = server
	cm.connPool = sync.Pool{New: func() any { return &serverConn{attr: &attr{}, connMgr: cm} }}
	cm.taskPool = sync.Pool{New: func() any { return &task{} }}
	cm.partitions = make([]*partition, runtime.NumCPU()*2)

	for i := 0; i < len(cm.partitions); i++ {
		cm.partitions[i] = &partition{connections: make(map[net.Conn]*serverConn)}
	}

	return cm
}

// close 关闭连接
func (cm *serverConnMgr) close() {
	wg, _ := taskpool.WithContext(context.Background())

	for _, p := range cm.partitions {
		wg.Go(p.close)
	}

	wg.Wait()
}

// allocateConn 分配连接
// @param c net.Conn TCP连接
// @return @1 error 错误信息
func (cm *serverConnMgr) allocateConn(c net.Conn) error {
	maxConnNum := int64(cm.server.opts.maxConnNum)
	for {
		if total := cm.total.Load(); total >= maxConnNum {
			return errors.ErrTooManyConnection
		} else if cm.total.CompareAndSwap(total, total+1) {
			break
		}
	}

	conn := cm.connPool.Get().(*serverConn)
	index := int(uintptr(unsafe.Pointer(c.(*net.TCPConn)))) % len(cm.partitions)
	cm.partitions[index].store(c, conn)
	conn.init(c)

	return nil
}

// recycleConn 回收连接
// @param c net.Conn TCP连接
func (cm *serverConnMgr) recycleConn(c net.Conn) {
	index := int(uintptr(unsafe.Pointer(c.(*net.TCPConn)))) % len(cm.partitions)
	if conn, ok := cm.partitions[index].delete(c); ok {
		conn.reset()
		cm.connPool.Put(conn)
		cm.total.Add(-1)
	}
}

// allocateTask 分配任务对象
// @param typ int8 任务类型
// @param msg ...[]byte 消息内容
// @return @1 *task 任务对象
func (cm *serverConnMgr) allocateTask(typ int8, msg ...[]byte) *task {
	t := cm.taskPool.Get().(*task)
	t.typ = typ
	if len(msg) > 0 {
		t.msg = msg[0]
	}

	return t
}

// recycleTask 回收任务到对象池
// @param t *task 任务对象
func (cm *serverConnMgr) recycleTask(t *task) {
	t.msg = nil
	cm.taskPool.Put(t)
}

type partition struct {
	rw          sync.RWMutex
	connections map[net.Conn]*serverConn
}

// store 存储连接
// @param c net.Conn TCP连接
// @param conn *serverConn 服务端连接
func (p *partition) store(c net.Conn, conn *serverConn) {
	p.rw.Lock()
	p.connections[c] = conn
	p.rw.Unlock()
}

// delete 删除连接
// @param c net.Conn TCP连接
// @return @1 *serverConn 服务端连接
// @return @2 bool 是否存在
func (p *partition) delete(c net.Conn) (*serverConn, bool) {
	p.rw.Lock()
	conn, ok := p.connections[c]
	if ok {
		delete(p.connections, c)
	}
	p.rw.Unlock()

	return conn, ok
}

// close 关闭该分片内的所有连接
// @return @1 error 错误信息
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
