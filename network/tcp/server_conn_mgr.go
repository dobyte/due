package tcp

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	taskpool "github.com/dobyte/due/v2/task"
	"github.com/pires/go-proxyproto"
)

type serverConnMgr struct {
	cid        atomic.Int64 // 连接ID
	total      atomic.Int64 // 总连接数
	server     *server      // 服务器
	connPool   sync.Pool    // 连接池
	partitions []*partition // 连接管理
}

// newServerConnMgr 创建连接管理器
// 初始化连接池、任务池以及按 CPU 数量动态扩容的分片结构
// @param server *server 所属服务器
// @return @1 *serverConnMgr 连接管理器
func newServerConnMgr(server *server) *serverConnMgr {
	cm := &serverConnMgr{}
	cm.server = server
	cm.connPool = sync.Pool{New: func() any {
		return &serverConn{
			attr:       &attr{},
			connMgr:    cm,
			reader:     bufio.NewReaderSize(nil, server.opts.readBufferSize),
			dueBuffers: make([]buffer.Buffer, 0, maxBatchWriteNum),
			netBuffers: make(net.Buffers, 0, 2*maxBatchWriteNum),
		}
	}}
	cm.partitions = make([]*partition, runtime.NumCPU()*2)

	for i := 0; i < len(cm.partitions); i++ {
		cm.partitions[i] = &partition{connections: make(map[net.Conn]*serverConn)}
	}

	return cm
}

// close 关闭所有连接
// 并行遍历所有分片，逐个关闭其中的连接并等待完成
func (cm *serverConnMgr) close() {
	wg, _ := taskpool.WithContext(context.Background())

	for _, p := range cm.partitions {
		wg.Go(p.close)
	}

	if err := wg.Wait(); err != nil {
		log.Warnf("close connections error: %v", err)
	}
}

// open 开放连接接入
// 清除所有分片的停止标志，服务器每次启动时调用以支持重启
func (cm *serverConnMgr) open() {
	for _, p := range cm.partitions {
		p.rw.Lock()
		p.stopped = false
		p.rw.Unlock()
	}
}

// allocateConn 分配连接
// 自增总连接数并校验上限，超限则回退计数；从连接池取用连接对象初始化后存入分片
// @param c net.Conn TCP连接
// @return @1 error 连接数已达上限或服务器已停止时返回的错误
func (cm *serverConnMgr) allocateConn(c net.Conn) error {
	if cm.total.Add(1) > int64(cm.server.opts.maxConnNum) {
		cm.total.Add(-1)
		return errors.ErrTooManyConnection
	}

	conn := cm.connPool.Get().(*serverConn)

	if !conn.init(c) {
		// 服务器关闭过程中分片拒绝存储，回退计数后归还连接对象，底层连接由调用方关闭
		cm.total.Add(-1)
		cm.connPool.Put(conn)
		return errors.ErrServerClosed
	}

	return nil
}

// storeConn 存储连接
// 按连接指针哈希存入对应分片，分片已停止时拒绝存储
// @param c net.Conn TCP连接
// @param conn *serverConn 对应的连接对象
// @return @1 bool 是否存储成功，服务器关闭过程中返回false
func (cm *serverConnMgr) storeConn(c net.Conn, conn *serverConn) bool {
	return cm.partitions[cm.connHash(c)].store(c, conn)
}

// recycleConn 回收连接
// 从分片中移除连接对象，重置后归还连接池并递减总连接数
// @param c net.Conn TCP连接
func (cm *serverConnMgr) recycleConn(c net.Conn) {
	if conn, ok := cm.partitions[cm.connHash(c)].delete(c); ok {
		conn.reset()
		cm.connPool.Put(conn)
		cm.total.Add(-1)
	}
}

// connHash 通过连接指针计算哈希
// 对连接对象指针地址做位混合后取模，确定其所属分片索引，避免对象地址对齐导致分片分布不均
// @param c net.Conn TCP连接
// @return @1 int 分片索引
func (cm *serverConnMgr) connHash(c net.Conn) int {
	var p uintptr

	switch cc := c.(type) {
	case *proxyproto.Conn:
		p = uintptr(unsafe.Pointer(cc))
	case *tls.Conn:
		p = uintptr(unsafe.Pointer(cc))
	case *net.TCPConn:
		p = uintptr(unsafe.Pointer(cc))
	default:
		if v := reflect.ValueOf(c); v.Kind() == reflect.Ptr {
			p = v.Pointer()
		}
		// 非指针类型的连接实现无法取得稳定地址，保持零值归入首个分片
	}

	return int(cm.mixPointer(p) % uintptr(len(cm.partitions)))
}

// mixPointer 打散指针地址，避免对象地址低位对齐导致取模后分片分布不均
func (cm *serverConnMgr) mixPointer(p uintptr) uintptr {
	x := uint64(p)
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33

	return uintptr(x)
}

type partition struct {
	rw          sync.RWMutex
	connections map[net.Conn]*serverConn
	stopped     bool     // 是否已停止接入新连接，关闭分片时置位，服务器重启时复位
	_           [31]byte // 填充至64字节缓存行，避免相邻分片伪共享
}

// store 存储连接
// 将连接映射写入分片；分片已停止时拒绝写入，避免服务器关闭过程中的在途连接泄漏
// @param c net.Conn TCP连接
// @param conn *serverConn 对应的连接对象
// @return @1 bool 是否存储成功，分片已停止时返回false
func (p *partition) store(c net.Conn, conn *serverConn) bool {
	p.rw.Lock()

	if p.stopped {
		p.rw.Unlock()
		return false
	}

	p.connections[c] = conn
	p.rw.Unlock()

	return true
}

// delete 删除连接
// 从分片中移除并返回对应的连接对象
// @param c net.Conn TCP连接
// @return @1 *serverConn 对应的连接对象，不存在时为nil
// @return @2 bool 连接是否存在
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
// 先置位停止标志阻断新连接写入，再串行关闭分片下所有连接，分片之间由外层并行驱动，
// 避免向任务池瞬时提交海量阻塞任务；连接被其他路径并发关闭属正常竞态，不视为错误
// @return @1 error 任一连接关闭失败时返回的首个错误
func (p *partition) close() error {
	p.rw.Lock()
	p.stopped = true
	conns := make([]*serverConn, 0, len(p.connections))
	for _, conn := range p.connections {
		conns = append(conns, conn)
	}
	p.rw.Unlock()

	var firstErr error

	for _, conn := range conns {
		err := conn.Close()
		if err == nil || errors.Is(err, errors.ErrConnectionNotOpened) || errors.Is(err, errors.ErrConnectionClosed) {
			continue
		}

		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
