package client

import "sync"

type pending struct {
	partitions []*partition // 分片
}

func newPending() *pending {
	p := &pending{partitions: make([]*partition, 20)}

	for i := 0; i < len(p.partitions); i++ {
		p.partitions[i] = &partition{calls: make(map[uint64]chan []byte)}
	}

	return p
}

// 提取
func (p *pending) extract(seq uint64) (chan []byte, bool) {
	return p.partitions[int(seq%uint64(len(p.partitions)))].extract(seq)
}

// 存储
func (p *pending) store(seq uint64, call chan []byte) {
	p.partitions[int(seq%uint64(len(p.partitions)))].store(seq, call)
}

// 删除
func (p *pending) delete(seq uint64) {
	p.partitions[int(seq%uint64(len(p.partitions)))].delete(seq)
}

type partition struct {
	mu    sync.Mutex             // 锁
	calls map[uint64]chan []byte // 同步通道
}

// 提取
func (p *partition) extract(seq uint64) (chan []byte, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	call, ok := p.calls[seq]
	if ok {
		delete(p.calls, seq)
	}

	return call, ok
}

// 存储
func (p *partition) store(seq uint64, call chan []byte) {
	p.mu.Lock()
	p.calls[seq] = call
	p.mu.Unlock()
}

// 删除
func (p *partition) delete(seq uint64) {
	p.mu.Lock()
	delete(p.calls, seq)
	p.mu.Unlock()
}
