package buffer

type Whence int

const (
	Head Whence = iota // 头部
	Tail               // 尾部
)

type Buffer struct {
	len  int
	head *NocopyNode
	tail *NocopyNode
}

func NewBuffer() *Buffer {
	return &Buffer{len: -1}
}

// Len 获取字节长度
func (b *Buffer) Len() int {
	if b.len >= 0 {
		return b.len
	}

	n := 0
	for node := b.head; node != nil; {
		n += node.Len()
		node = node.next
	}
	b.len = n

	return n
}

// Range 迭代
func (b *Buffer) Range(fn func(node *NocopyNode) bool) {
	node := b.head
	for node != nil {
		next := node.next

		if fn(node) {
			node = next
		} else {
			break
		}
	}
}

// Malloc 分配一块内存给Writer
func (b *Buffer) Malloc(cap int, whence ...Whence) *Writer {
	writer := NewWriter(cap)

	if len(whence) > 0 && whence[0] == Head {
		b.addToHead(&NocopyNode{buff: writer})
	} else {
		b.addToTail(&NocopyNode{buff: writer})
	}

	return writer
}

// Mount 挂载数据到Buffer上
func (b *Buffer) Mount(data []byte, whence ...Whence) {
	if len(whence) > 0 && whence[0] == Head {
		b.addToHead(&NocopyNode{buff: data})
	} else {
		b.addToTail(&NocopyNode{buff: data})
	}
}

// Release 释放
func (b *Buffer) Release() {
	node := b.head
	for node != nil {
		next := node.next
		node.Release()
		node = next
	}
	b.head = nil
	b.tail = nil
}

// 添加到尾部
func (b *Buffer) addToTail(node *NocopyNode) {
	if b.head == nil {
		b.head = node
		b.tail = node
	} else {
		b.tail.next = node
		b.tail.next.prev = b.tail
		b.tail = node
	}

	b.len = -1
}

// 添加到头部
func (b *Buffer) addToHead(node *NocopyNode) {
	if b.head == nil {
		b.head = node
		b.tail = node
	} else {
		node.next = b.head
		b.head.prev = node
		b.head = node
	}

	b.len = -1
}
