package quic

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/quic-go/quic-go"
)

type managedConn struct {
	qc   *quic.Conn
	conn *serverConn
}

type partition struct {
	mu          sync.Mutex
	connections map[int64]managedConn
	_           [48]byte // 填充至64字节缓存行，避免相邻分片伪共享
}

// serverConnMgr 服务器连接管理器
// 跟踪挂起的流与活跃的连接，并通过 sync.Pool 复用服务器连接对象
type serverConnMgr struct {
	server     *server
	cid        atomic.Int64
	total      atomic.Int64
	connPool   sync.Pool
	partitions []partition
	closed     atomic.Bool
	closeOnce  sync.Once
}

func newServerConnMgr(s *server) *serverConnMgr {
	m := &serverConnMgr{
		server:     s,
		partitions: make([]partition, max(1, runtime.GOMAXPROCS(0)*2)),
	}
	for i := range m.partitions {
		m.partitions[i].connections = make(map[int64]managedConn)
	}
	m.connPool = sync.Pool{New: func() any {
		return &serverConn{
			attr:       &attr{},
			connMgr:    m,
			dueBuffers: make([]buffer.Buffer, 0, maxBatchWriteNum),
		}
	}}
	return m
}

// reserve 预留连接
// 分配连接ID并为挂起的流计数；管理器已关闭或达到最大连接数时预留失败
// @param qc *quic.Conn QUIC连接
// @return @1 int64 预留的连接ID
// @return @2 bool 是否预留成功
func (m *serverConnMgr) reserve(qc *quic.Conn) (int64, bool) {
	for {
		total := m.total.Load()
		if m.closed.Load() || total >= int64(m.server.opts.maxConnNum) {
			return 0, false
		}
		if m.total.CompareAndSwap(total, total+1) {
			break
		}
	}

	id := m.cid.Add(1)
	p := &m.partitions[uint64(id)%uint64(len(m.partitions))]
	p.mu.Lock()
	defer p.mu.Unlock()
	if m.closed.Load() {
		m.total.Add(-1)
		return 0, false
	}
	p.connections[id] = managedConn{qc: qc}
	return id, true
}

// allocateConn 分配连接
// 将池化连接对象挂接到预留条目上并完成初始化。
// 初始化中止（管理器已关闭或预留条目已被清理）时归还连接对象并返回nil，
// 预留条目由 close/remove 路径负责清理与计数回退，底层连接由调用方关闭
// @param id int64 预留的连接ID
// @param qc *quic.Conn QUIC连接
// @param stream *quic.Stream 双向流
// @return @1 *serverConn 连接对象，分配失败时返回nil
func (m *serverConnMgr) allocateConn(id int64, qc *quic.Conn, stream *quic.Stream) *serverConn {
	c := m.connPool.Get().(*serverConn)

	if !c.init(id, qc, stream) {
		m.connPool.Put(c)
		return nil
	}

	return c
}

// linkConn 将池化连接对象挂接到预留条目上
// 仅在条目存在且管理器未关闭时挂接成功；须由连接对象在持有自身写锁时调用，
// 挂接成功后关闭路径即可感知该连接对象，与初始化严格串行
// @param id int64 预留的连接ID
// @param c *serverConn 池化连接对象
// @return @1 bool 是否挂接成功
func (m *serverConnMgr) linkConn(id int64, c *serverConn) bool {
	p := &m.partitions[uint64(id)%uint64(len(m.partitions))]
	p.mu.Lock()
	defer p.mu.Unlock()

	entry, ok := p.connections[id]
	if !ok || m.closed.Load() {
		return false
	}

	entry.conn = c
	p.connections[id] = entry

	return true
}

// remove 移除预留条目
// 删除流接入失败的预留条目并回退计数
// @param id int64 预留的连接ID
func (m *serverConnMgr) remove(id int64) {
	p := &m.partitions[uint64(id)%uint64(len(m.partitions))]
	p.mu.Lock()
	if _, ok := p.connections[id]; ok {
		delete(p.connections, id)
		m.total.Add(-1)
	}
	p.mu.Unlock()
}

// recycleConn 回收连接
// 移除活跃连接并将连接对象归还对象池
// @param c *serverConn 连接对象
func (m *serverConnMgr) recycleConn(c *serverConn) {
	p := &m.partitions[uint64(c.id)%uint64(len(m.partitions))]
	p.mu.Lock()
	if _, ok := p.connections[c.id]; ok {
		delete(p.connections, c.id)
		m.total.Add(-1)
	}
	p.mu.Unlock()

	c.reset()
	m.connPool.Put(c)
}

// close 关闭连接管理器
// 停止所有挂起的流与活跃的连接
func (m *serverConnMgr) close() {
	m.closeOnce.Do(func() {
		m.closed.Store(true)
		var wg sync.WaitGroup
		for i := range m.partitions {
			p := &m.partitions[i]
			wg.Add(1)
			go func() {
				defer wg.Done()
				p.mu.Lock()
				entries := make([]managedConn, 0, len(p.connections))
				for id, entry := range p.connections {
					entries = append(entries, entry)
					delete(p.connections, id)
					m.total.Add(-1)
				}
				p.mu.Unlock()
				for _, entry := range entries {
					if entry.conn != nil {
						_ = entry.conn.forceClose()
					} else {
						_ = entry.qc.CloseWithError(0, "server stopped")
					}
				}
			}()
		}
		wg.Wait()
	})
}
