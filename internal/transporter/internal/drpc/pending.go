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
		p.calls[i] = &calls{calls: make(map[uint64]chan *buffer.Bytes)}
	}

	return p
}

// 回复
func (p *pending) reply(seq uint64, buf *buffer.Bytes) bool {
	return p.calls[int(seq%uint64(len(p.calls)))].reply(seq, buf)
}

// 存储
func (p *pending) store(seq uint64, ch chan *buffer.Bytes) {
	p.calls[int(seq%uint64(len(p.calls)))].store(seq, ch)
}

// 删除
func (p *pending) delete(seq uint64) bool {
	return p.calls[int(seq%uint64(len(p.calls)))].delete(seq)
}

// closeAll 关闭所有等待中的调用，唤醒全部等待者并释放资源
func (p *pending) closeAll() {
	for _, c := range p.calls {
		c.closeAll()
	}
}

type calls struct {
	mu    sync.Mutex                    // 锁
	calls map[uint64]chan *buffer.Bytes // 同步通道
}

// 提取
func (p *calls) reply(seq uint64, buf *buffer.Bytes) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ch, ok := p.calls[seq]; ok {
		delete(p.calls, seq)

		select {
		case ch <- buf:
			return true
		default:
			return false
		}
	} else {
		return false
	}
}

// 存储
func (p *calls) store(seq uint64, ch chan *buffer.Bytes) {
	p.mu.Lock()
	p.calls[seq] = ch
	p.mu.Unlock()
}

// 删除
func (p *calls) delete(seq uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ch, ok := p.calls[seq]; ok {
		close(ch)

		delete(p.calls, seq)

		return true
	} else {
		return false
	}
}

// closeAll 关闭所有等待中的调用
func (p *calls) closeAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for seq, ch := range p.calls {
		close(ch)
		delete(p.calls, seq)
	}
}
