package kcp

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	taskpool "github.com/dobyte/due/v2/task"
	"github.com/xtaci/kcp-go/v5"
)

type serverConnMgr struct {
	id         atomic.Int64 // 连接ID
	total      atomic.Int64 // 总连接数
	server     *server      // 服务器
	connPool   sync.Pool    // 连接池
	taskPool   sync.Pool    // 任务池
	partitions []*partition // 连接管理
}

// newServerConnMgr 创建连接管理器
// 初始化连接池、任务池和按CPU数分片的分片管理器
// @param server *server 服务器实例
// @return @1 *serverConnMgr 连接管理器
func newServerConnMgr(server *server) *serverConnMgr {
	cm := &serverConnMgr{}
	cm.server = server
	cm.connPool = sync.Pool{New: func() any { return &serverConn{attr: &attr{}, connMgr: cm} }}
	cm.taskPool = sync.Pool{New: func() any { return &task{} }}
	cm.partitions = make([]*partition, runtime.NumCPU()*2)

	for i := 0; i < len(cm.partitions); i++ {
		cm.partitions[i] = &partition{connections: make(map[*kcp.UDPSession]*serverConn)}
	}

	return cm
}

// close 关闭连接
// 并发关闭所有分片内的连接
func (cm *serverConnMgr) close() {
	wg, _ := taskpool.WithContext(context.Background())

	for _, p := range cm.partitions {
		wg.Go(p.close)
	}

	wg.Wait()
}

// allocateConn 分配连接
// 通过CAS校验连接数上限后从连接池取出连接并初始化
// @param c *kcp.UDPSession KCP连接
// @return @1 error 连接数达到上限时返回errors.ErrTooManyConnection
func (cm *serverConnMgr) allocateConn(c *kcp.UDPSession) error {
	maxConnNum := int64(cm.server.opts.maxConnNum)
	for {
		if total := cm.total.Load(); total >= maxConnNum {
			return errors.ErrTooManyConnection
		} else if cm.total.CompareAndSwap(total, total+1) {
			break
		}
	}

	conn := cm.connPool.Get().(*serverConn)
	conn.init(c)

	return nil
}

// storeConn 存储连接
// 将连接按哈希分片存储到对应分片
// @param c *kcp.UDPSession KCP连接
// @param conn *serverConn 服务器连接对象
func (cm *serverConnMgr) storeConn(c *kcp.UDPSession, conn *serverConn) {
	cm.partitions[cm.connHash(c)].store(c, conn)
}

// recycleConn 回收连接
// 从分片中删除连接、重置并归还连接池，同时递减总连接数
// @param c *kcp.UDPSession KCP连接
func (cm *serverConnMgr) recycleConn(c *kcp.UDPSession) {
	index := cm.connHash(c)
	if conn, ok := cm.partitions[index].delete(c); ok {
		conn.reset()
		cm.connPool.Put(conn)
		cm.total.Add(-1)
	}
}

// allocateTask 分配任务对象
// 从对象池中取出任务并填充类型与消息内容
// @param typ int8 任务类型
// @param msg ...[]byte 待发送的消息字节，可选
// @return @1 *task 分配到的任务对象
func (cm *serverConnMgr) allocateTask(typ int8, msg ...[]byte) *task {
	t := cm.taskPool.Get().(*task)
	t.typ = typ
	if len(msg) > 0 {
		t.msg = msg[0]
	}

	return t
}

// recycleTask 回收任务到对象池
// 清空消息内容后将任务归还对象池，以便复用
// @param t *task 待回收的任务对象
func (cm *serverConnMgr) recycleTask(t *task) {
	t.msg = nil
	cm.taskPool.Put(t)
}

// connHash 通过连接指针计算哈希
// 根据连接对象指针地址取模确定其所属分片索引
// @param c *kcp.UDPSession KCP连接
// @return @1 int 分片索引
func (cm *serverConnMgr) connHash(c *kcp.UDPSession) int {
	return int(uintptr(unsafe.Pointer(c)) % uintptr(len(cm.partitions)))
}

type partition struct {
	rw          sync.RWMutex
	connections map[*kcp.UDPSession]*serverConn
}

// store 存储连接
// @param c *kcp.UDPSession KCP连接
// @param conn *serverConn 服务器连接对象
func (p *partition) store(c *kcp.UDPSession, conn *serverConn) {
	p.rw.Lock()
	p.connections[c] = conn
	p.rw.Unlock()
}

// delete 删除连接
// @param c *kcp.UDPSession KCP连接
// @return @1 *serverConn 被删除的连接对象
// @return @2 bool 是否存在对应的连接
func (p *partition) delete(c *kcp.UDPSession) (*serverConn, bool) {
	p.rw.Lock()
	conn, ok := p.connections[c]
	if ok {
		delete(p.connections, c)
	}
	p.rw.Unlock()

	return conn, ok
}

// close 关闭该分片内的所有连接
// @return @1 error 关闭连接的聚合错误
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
