package buffer

type NocopyNode struct {
	prev  any
	next  any
	block any
}

var _ Buffer = (*NocopyNode)(nil)

// Len 获取字节长度
func (n *NocopyNode) Len() int {
	if n == nil {
		return 0
	}

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

// Bytes 获取该节点的字节数据
func (n *NocopyNode) Bytes() []byte {
	if n == nil {
		return nil
	}

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
	if n == nil {
		return
	}

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
