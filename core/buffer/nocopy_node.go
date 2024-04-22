package buffer

type NocopyNode struct {
	buff any
	prev *NocopyNode
	next *NocopyNode
}

// Release 释放
func (n *NocopyNode) Release() {
	n.prev = nil
	n.next = nil
	n.buff = nil
}

// Len 获取字节长度
func (n *NocopyNode) Len() int {
	if n.buff == nil {
		return 0
	}

	switch b := n.buff.(type) {
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
	if n.buff == nil {
		return nil
	}

	switch b := n.buff.(type) {
	case []byte:
		return b
	case *Writer:
		return b.Bytes()
	default:
		return nil
	}
}
