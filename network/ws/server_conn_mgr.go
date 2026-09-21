/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/28 3:48 下午
 * @Desc: 连接管理器
 */

package ws

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
	"github.com/gorilla/websocket"
)

type serverConnMgr struct {
	cid        atomic.Int64 // 连接ID
	total      atomic.Int64 // 总连接数
	server     *server      // 服务器
	connPool   sync.Pool    // 连接池
	partitions []*partition // 连接管理
}

// newConnMgr 创建连接管理器
// 初始化连接池、任务池以及按 CPU 数量动态扩容的分片结构
// @param server *server 所属服务器
// @return @1 *serverConnMgr 连接管理器
func newConnMgr(server *server) *serverConnMgr {
	cm := &serverConnMgr{}
	cm.server = server
	cm.connPool = sync.Pool{New: func() any { return &serverConn{attr: &attr{}, connMgr: cm} }}
	cm.partitions = make([]*partition, runtime.NumCPU()*2)

	for i := 0; i < len(cm.partitions); i++ {
		cm.partitions[i] = &partition{connections: make(map[*websocket.Conn]*serverConn)}
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

	wg.Wait()
}

// allocateConn 分配连接
// 自增总连接数并校验上限，从连接池取用连接对象存入分片后完成初始化
// @param c *websocket.Conn WS连接
// @param remoteAddr net.Addr 客户端真实地址，应用层代理模式下从代理头解析得到，可为nil
// @return @1 error 连接数已达上限时返回errors.ErrTooManyConnection
func (cm *serverConnMgr) allocateConn(c *websocket.Conn, remoteAddr net.Addr) error {
	maxConnNum := int64(cm.server.opts.maxConnNum)
	for {
		if total := cm.total.Load(); total >= maxConnNum {
			return errors.ErrTooManyConnection
		} else if cm.total.CompareAndSwap(total, total+1) {
			break
		}
	}

	conn := cm.connPool.Get().(*serverConn)
	conn.init(c, remoteAddr)

	return nil
}

// 存储连接
func (cm *serverConnMgr) storeConn(c *websocket.Conn, conn *serverConn) {
	cm.partitions[cm.connHash(c)].store(c, conn)
}

// recycleConn 回收连接
// 从分片中移除连接对象，重置后归还连接池并递减总连接数
// @param c *websocket.Conn WS连接
func (cm *serverConnMgr) recycleConn(c *websocket.Conn) {
	if conn, ok := cm.partitions[cm.connHash(c)].delete(c); ok {
		conn.reset()
		cm.connPool.Put(conn)
		cm.total.Add(-1)
	}
}

// connHash 通过连接指针计算哈希
// 根据连接对象指针地址取模确定其所属分片索引
// @param c *websocket.Conn WS连接
// @return @1 int 分片索引
func (cm *serverConnMgr) connHash(c *websocket.Conn) int {
	return int(uintptr(unsafe.Pointer(c))) % len(cm.partitions)
}

type partition struct {
	rw          sync.RWMutex
	connections map[*websocket.Conn]*serverConn
}

// store 存储连接
// 将连接映射写入分片
// @param c *websocket.Conn WS连接
// @param conn *serverConn 对应的连接对象
func (p *partition) store(c *websocket.Conn, conn *serverConn) {
	p.rw.Lock()
	p.connections[c] = conn
	p.rw.Unlock()
}

// delete 删除连接
// 从分片中移除并返回对应的连接对象
// @param c *websocket.Conn WS连接
// @return @1 *serverConn 对应的连接对象，不存在时为nil
// @return @2 bool 连接是否存在
func (p *partition) delete(c *websocket.Conn) (*serverConn, bool) {
	p.rw.Lock()
	conn, ok := p.connections[c]
	if ok {
		delete(p.connections, c)
	}
	p.rw.Unlock()

	return conn, ok
}

// close 关闭该分片内的所有连接
// 并发关闭分片下所有连接并等待完成
// @return @1 error 任一连接关闭失败时返回的错误
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

// genConnID 生成连接ID
// @return @1 int64 连接ID
func (cm *serverConnMgr) genConnID() int64 {
	if cid := cm.cid.Add(1); cid == 0 {
		return cm.cid.Add(1)
	} else {
		return cid
	}
}
