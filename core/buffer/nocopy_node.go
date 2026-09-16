package buffer

// NocopyNode 零拷贝缓冲区节点
type NocopyNode struct {
	prev  any
	next  any
	block any
}

// Len 获取字节长度
func (n *NocopyNode) Len() int {
	switch b := n.block.(type) {
	case []byte:
		return len(b)
	case *Bytes:
		return b.Len()
	case *Writer:
		return b.Len()
	default:
		return 0
	}
}

// Nodes 获取节点数
func (n *NocopyNode) Nodes() int {
	return 1
}

// Bytes 获取该节点的字节数据
func (n *NocopyNode) Bytes() []byte {
	switch b := n.block.(type) {
	case []byte:
		return b
	case *Bytes:
		return b.Bytes()
	case *Writer:
		return b.Bytes()
	default:
		return nil
	}
}

// Release 释放
func (n *NocopyNode) Release() {
	switch b := n.block.(type) {
	case []byte:
		// ignore
	case *Bytes:
		b.Release()
	case *Writer:
		b.Release()
	}

	n.prev = nil
	n.next = nil
	n.block = nil
}
