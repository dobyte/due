package kcp

import (
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/xtaci/kcp-go/v5"
)

type serverConnMgr struct {
	id         int64        // 连接ID
	total      int64        // 总连接数
	server     *server      // 服务器
	pool       sync.Pool    // 连接池
	partitions []*partition // 连接管理
}

func newServerConnMgr(server *server) *serverConnMgr {
	cm := &serverConnMgr{}
	cm.server = server
	cm.pool = sync.Pool{New: func() any { return &serverConn{} }}
	cm.partitions = make([]*partition, 100)

	for i := 0; i < len(cm.partitions); i++ {
		cm.partitions[i] = &partition{connections: make(map[*kcp.UDPSession]*serverConn)}
	}

	return cm
}

// 关闭连接
func (cm *serverConnMgr) close() {
	var wg sync.WaitGroup

	wg.Add(len(cm.partitions))

	for i := range cm.partitions {
		p := cm.partitions[i]

		xcall.Go(func() {
			p.close()
			wg.Done()
		})
	}

	wg.Wait()
}

// 分配连接
func (cm *serverConnMgr) allocate(c *kcp.UDPSession) error {
	if atomic.LoadInt64(&cm.total) >= int64(cm.server.opts.maxConnNum) {
		return errors.ErrTooManyConnection
	}

	id := atomic.AddInt64(&cm.id, 1)
	conn := cm.pool.Get().(*serverConn)
	conn.init(cm, id, c)
	index := int(reflect.ValueOf(c).Pointer()) % len(cm.partitions)
	cm.partitions[index].store(c, conn)
	atomic.AddInt64(&cm.total, 1)

	return nil
}

// 回收连接
func (cm *serverConnMgr) recycle(c *kcp.UDPSession) {
	index := int(reflect.ValueOf(c).Pointer()) % len(cm.partitions)
	if conn, ok := cm.partitions[index].delete(c); ok {
		cm.pool.Put(conn)
		atomic.AddInt64(&cm.total, -1)
	}
}

type partition struct {
	rw          sync.RWMutex
	connections map[*kcp.UDPSession]*serverConn
}

// 存储连接
func (p *partition) store(c *kcp.UDPSession, conn *serverConn) {
	p.rw.Lock()
	p.connections[c] = conn
	p.rw.Unlock()
}

// 加载连接
func (p *partition) load(c *kcp.UDPSession) (*serverConn, bool) {
	p.rw.RLock()
	conn, ok := p.connections[c]
	p.rw.RUnlock()

	return conn, ok
}

// 删除连接
func (p *partition) delete(c *kcp.UDPSession) (*serverConn, bool) {
	p.rw.Lock()
	conn, ok := p.connections[c]
	if ok {
		delete(p.connections, c)
	}
	p.rw.Unlock()

	return conn, ok
}

// 关闭该分片内的所有连接
func (p *partition) close() {
	for _, conn := range p.connections {
		_ = conn.Close()
	}
}
