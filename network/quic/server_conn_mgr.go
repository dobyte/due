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
	_           [48]byte // Padding to a 64-byte cache line to avoid false sharing between neighboring partitions
}

// serverConnMgr is a server connection manager.
//
// It tracks pending streams and active connections and reuses server connection objects through a
// sync.Pool.
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

// reserve reserves a connection.
//
// It allocates a connection ID and counts the pending stream. Reservation fails when the manager
// has been closed or the maximum number of connections has been reached. It returns the reserved
// connection ID and whether the reservation succeeded.
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

// allocateConn allocates a connection.
//
// It attaches a pooled connection object to the reserved entry and initializes it. When
// initialization is aborted, because the manager has been closed or the reserved entry has already
// been cleaned up, it returns the connection object and yields nil; the reserved entry is cleaned
// up and its count rolled back by the close and remove paths, and the caller closes the underlying
// connection.
func (m *serverConnMgr) allocateConn(id int64, qc *quic.Conn, stream *quic.Stream) *serverConn {
	c := m.connPool.Get().(*serverConn)

	if !c.init(id, qc, stream) {
		m.connPool.Put(c)
		return nil
	}

	return c
}

// linkConn attaches a pooled connection object to the reserved entry.
//
// It succeeds only when the entry exists and the manager has not been closed. It must be called by
// the connection object while holding its own write lock; once the link succeeds the close path can
// see the connection object, which keeps the operation strictly serialized with initialization. It
// reports whether the link succeeded.
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

// remove removes a reserved entry.
//
// It deletes the reserved entry of a stream that failed to attach and rolls the count back.
func (m *serverConnMgr) remove(id int64) {
	p := &m.partitions[uint64(id)%uint64(len(m.partitions))]
	p.mu.Lock()
	if _, ok := p.connections[id]; ok {
		delete(p.connections, id)
		m.total.Add(-1)
	}
	p.mu.Unlock()
}

// recycleConn recycles a connection.
//
// It removes the active connection and returns the connection object to the pool.
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

// close closes the connection manager.
//
// It stops every pending stream and active connection.
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
