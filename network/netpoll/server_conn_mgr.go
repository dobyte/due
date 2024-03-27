package netpoll

import (
	"github.com/cloudwego/netpoll"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xcall"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

const (
	partitions = 100
)

type connMgr struct {
	id         int64        // 连接ID
	total      int64        // 总连接数
	server     *server      // 服务器
	pool       sync.Pool    // 连接池
	partitions []*partition // 连接管理
}

func newConnMgr(server *server) *connMgr {
	cm := &connMgr{}
	cm.server = server
	cm.pool = sync.Pool{New: func() interface{} { return &serverConn{} }}
	cm.partitions = make([]*partition, partitions)

	cm.init()

	return cm
}

// 初始化
func (cm *connMgr) init() {
	for i := 0; i < partitions; i++ {
		cm.partitions[i] = &partition{connections: make(map[netpoll.Connection]*serverConn)}
	}

	xcall.Go(cm.heartbeat)
}

// 执行心跳
func (cm *connMgr) heartbeat() {
	if cm.server.opts.heartbeatMechanism != TickHeartbeat {
		return
	}

	if cm.server.opts.heartbeatInterval <= 0 {
		return
	}

	ticker := time.NewTicker(cm.server.opts.heartbeatInterval)

	for {
		<-ticker.C

		for i := range cm.partitions {
			p := cm.partitions[i]
			xcall.Go(p.heartbeat)
		}
	}
}

// 关闭连接
func (cm *connMgr) close() {
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

	index := int(reflect.ValueOf(conn.conn).Pointer()) % len(cm.partitions)
	cm.partitions[index].store(c, conn)
	atomic.AddInt64(&cm.total, 1)

	return nil
}

// 回收连接
func (cm *connMgr) recycle(c netpoll.Connection) {
	index := int(reflect.ValueOf(c).Pointer()) % len(cm.partitions)
	if conn, ok := cm.partitions[index].delete(c); ok {
		cm.pool.Put(conn)
		atomic.AddInt64(&cm.total, -1)
	}
}

// 加载连接
func (cm *connMgr) load(c netpoll.Connection) (*serverConn, bool) {
	index := int(reflect.ValueOf(c).Pointer()) % len(cm.partitions)
	return cm.partitions[index].load(c)
}

type partition struct {
	rw          sync.RWMutex
	connections map[netpoll.Connection]*serverConn
}

// 存储连接
func (p *partition) store(c netpoll.Connection, conn *serverConn) {
	p.rw.Lock()
	p.connections[c] = conn
	p.rw.Unlock()
}

// 加载连接
func (p *partition) load(c netpoll.Connection) (*serverConn, bool) {
	p.rw.RLock()
	conn, ok := p.connections[c]
	p.rw.RUnlock()

	return conn, ok
}

// 删除连接
func (p *partition) delete(c netpoll.Connection) (*serverConn, bool) {
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

// 出发该分片内的所有连接心跳
func (p *partition) heartbeat() {
	for _, conn := range p.connections {
		_ = conn.heartbeat()
	}
}
