package drpc

import (
	"sync"

	"github.com/dobyte/due/v2/core/buffer"
)

type pending struct {
	calls []*calls // 分片
}

func newPending() *pending {
	p := &pending{calls: make([]*calls, 64)}

	for i := 0; i < len(p.calls); i++ {
		p.calls[i] = &calls{calls: make(map[uint64]*call)}
	}

	return p
}

// 回复
func (p *pending) reply(seq uint64, buf buffer.Buffer) bool {
	return p.calls[int(seq%uint64(len(p.calls)))].reply(seq, buf)
}

// 存储
func (p *pending) store(seq uint64, ch chan buffer.Buffer, data []byte) {
	p.calls[int(seq%uint64(len(p.calls)))].store(seq, ch, data)
}

// 删除
func (p *pending) delete(seq uint64) bool {
	return p.calls[int(seq%uint64(len(p.calls)))].delete(seq)
}

// snapshot 获取所有未完成调用的快照，用于重连后重发
func (p *pending) snapshot() []*call {
	var out []*call

	for _, c := range p.calls {
		out = append(out, c.snapshot()...)
	}

	return out
}

// closeAll 关闭所有等待中的调用，唤醒全部等待者并释放资源
func (p *pending) closeAll() {
	for _, c := range p.calls {
		c.closeAll()
	}
}

// call 一次未完成的调用
type call struct {
	ch   chan buffer.Buffer // 响应通道
	data []byte             // 请求数据副本，用于重连后重发
}

type calls struct {
	mu    sync.Mutex       // 锁
	calls map[uint64]*call // 同步通道
}

// 提取
func (p *calls) reply(seq uint64, buf buffer.Buffer) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if c, ok := p.calls[seq]; ok {
		delete(p.calls, seq)

		select {
		case c.ch <- buf:
			return true
		default:
			return false
		}
	} else {
		return false
	}
}

// 存储
func (p *calls) store(seq uint64, ch chan buffer.Buffer, data []byte) {
	p.mu.Lock()
	p.calls[seq] = &call{ch: ch, data: data}
	p.mu.Unlock()
}

// 删除
func (p *calls) delete(seq uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if c, ok := p.calls[seq]; ok {
		close(c.ch)

		delete(p.calls, seq)

		return true
	} else {
		return false
	}
}

// snapshot 获取未完成调用快照
func (p *calls) snapshot() []*call {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := make([]*call, 0, len(p.calls))

	for _, c := range p.calls {
		out = append(out, c)
	}

	return out
}

// closeAll 关闭所有等待中的调用
func (p *calls) closeAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for seq, c := range p.calls {
		close(c.ch)
		delete(p.calls, seq)
	}
}
