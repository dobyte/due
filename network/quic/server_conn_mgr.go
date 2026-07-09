package quic

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	taskpool "github.com/dobyte/due/v2/task"
	"github.com/quic-go/quic-go"
)

type serverConnMgr struct {
	id         atomic.Int64 // 连接ID
	total      atomic.Int64 // 总连接数
	server     *server      // 服务器
	connPool   sync.Pool    // 连接池
	taskPool   sync.Pool    // 任务池
	partitions []*partition // 连接管理
}

func newServerConnMgr(server *server) *serverConnMgr {
	cm := &serverConnMgr{}
	cm.server = server
	cm.connPool = sync.Pool{New: func() any { return &serverConn{attr: &attr{}, connMgr: cm} }}
	cm.taskPool = sync.Pool{New: func() any { return &task{} }}
	cm.partitions = make([]*partition, runtime.NumCPU()*2)

	for i := 0; i < len(cm.partitions); i++ {
		cm.partitions[i] = &partition{connections: make(map[quic.Connection]*serverConn)}
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
func (cm *serverConnMgr) allocateConn(qc quic.Connection, stream quic.Stream) error {
	maxConnNum := int64(cm.server.opts.maxConnNum)
	for {
		if total := cm.total.Load(); total >= maxConnNum {
			return errors.ErrTooManyConnection
		} else if cm.total.CompareAndSwap(total, total+1) {
			break
		}
	}

	conn := cm.connPool.Get().(*serverConn)
	index := connHash(qc, len(cm.partitions))
	cm.partitions[index].store(qc, conn)
	conn.init(qc, stream)

	return nil
}

// 回收连接
func (cm *serverConnMgr) recycleConn(qc quic.Connection) {
	index := connHash(qc, len(cm.partitions))
	if conn, ok := cm.partitions[index].delete(qc); ok {
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

// 通过接口底层指针计算哈希
func connHash(qc quic.Connection, n int) int {
	return int((*iface)(unsafe.Pointer(&qc)).data % uintptr(n))
}

type partition struct {
	rw          sync.RWMutex
	connections map[quic.Connection]*serverConn
}

// 存储连接
func (p *partition) store(qc quic.Connection, conn *serverConn) {
	p.rw.Lock()
	p.connections[qc] = conn
	p.rw.Unlock()
}

// 删除连接
func (p *partition) delete(qc quic.Connection) (*serverConn, bool) {
	p.rw.Lock()
	conn, ok := p.connections[qc]
	if ok {
		delete(p.connections, qc)
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
