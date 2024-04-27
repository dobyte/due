package buffer

type NocopyNode struct {
	buf  any
	pool *WriterPool
	prev *NocopyNode
	next *NocopyNode
}

// Release 释放
func (n *NocopyNode) Release() {
	n.prev = nil
	n.next = nil

	switch b := n.buf.(type) {
	case []byte:
		n.buf = nil
	case *Writer:
		b.Reset()
		if n.pool != nil {
			n.pool.Put(b)
		}
	}
}

// Len 获取字节长度
func (n *NocopyNode) Len() int {
	if n.buf == nil {
		return 0
	}

	switch b := n.buf.(type) {
	case []byte:
		return len(b)
	case *Writer:
		return b.Len()
	default:
		return 0
	}
}

// Bytes 获取该节点的字节数据
func (n *NocopyNode) Bytes() []byte {
	switch b := n.buf.(type) {
	case []byte:
		return b
	case *Writer:
		return b.Bytes()
	default:
		return nil
	}
}

// Next 下一个节点
func (n *NocopyNode) Next() *NocopyNode {
	if n == nil {
		return nil
	}

	return n.next
}
