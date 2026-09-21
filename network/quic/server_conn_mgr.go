package quic

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/quic-go/quic-go"
)

type managedConn struct {
	qc   *quic.Conn
	conn *serverConn
}

type partition struct {
	mu          sync.Mutex
	connections map[int64]managedConn
}

// serverConnMgr tracks pending streams and active connections, reusing server
// connection objects through a sync.Pool.
type serverConnMgr struct {
	server     *server
	id         atomic.Int64
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
		return &serverConn{attr: &attr{}, connMgr: m}
	}}
	return m
}

// reserve allocates a connection ID and accounts for a pending stream.
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

	id := m.id.Add(1)
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

// allocateConn links a pooled connection to a reserved slot and initializes it.
func (m *serverConnMgr) allocateConn(id int64, qc *quic.Conn, stream *quic.Stream) *serverConn {
	p := &m.partitions[uint64(id)%uint64(len(m.partitions))]
	p.mu.Lock()
	entry, ok := p.connections[id]
	if !ok || m.closed.Load() {
		p.mu.Unlock()
		return nil
	}
	c := m.connPool.Get().(*serverConn)
	entry.conn = c
	p.connections[id] = entry
	p.mu.Unlock()

	c.init(id, qc, stream)

	return c
}

// remove deletes a pending slot whose stream failed to be accepted.
func (m *serverConnMgr) remove(id int64) {
	p := &m.partitions[uint64(id)%uint64(len(m.partitions))]
	p.mu.Lock()
	if _, ok := p.connections[id]; ok {
		delete(p.connections, id)
		m.total.Add(-1)
	}
	p.mu.Unlock()
}

// recycleConn removes an active connection and returns its object to the pool.
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

// close stops all pending streams and active connections.
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
						entry.conn.forceClose(false)
					} else {
						_ = entry.qc.CloseWithError(0, "server stopped")
					}
				}
			}()
		}
		wg.Wait()
	})
}
