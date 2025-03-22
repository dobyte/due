package network

import (
	"sync"
)

type connTaskPool struct {
	pool sync.Pool
}

var TaskPool = NewTaskPool()

func NewTaskPool() *connTaskPool {
	return &connTaskPool{pool: sync.Pool{New: func() interface{} { return ConnTask{} }}}
}

func (p *connTaskPool) Get(conn Conn, msg []byte) ConnTask {
	t := p.pool.Get().(ConnTask)
	t.Reset(conn, msg)
	return t
}

func (p *connTaskPool) Put(t ConnTask) {
	t.msg = nil
	t.conn = nil
	p.pool.Put(t)
}

type ConnTask struct {
	conn Conn
	msg  []byte
}

func (t *ConnTask) GetConn() Conn {
	return t.conn
}
func (t *ConnTask) GetMsg() []byte {
	return t.msg
}
func (t *ConnTask) Reset(conn Conn, msg []byte) {
	t.msg = msg
	t.conn = conn
}

// 避免如果在worker中新开一个协程去执行任务时,影响原始的执行过程
func (t *ConnTask) Copy() *ConnTask {
	return &ConnTask{
		msg:  t.msg,
		conn: t.conn,
	}
}
