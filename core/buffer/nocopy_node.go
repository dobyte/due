package buffer

// NocopyNode is a zero-copy buffer node.
type NocopyNode struct {
	next  any
	block any
}

// Len returns the byte length.
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

// Nodes returns the number of nodes.
func (n *NocopyNode) Nodes() int {
	return 1
}

// Bytes returns the byte data of the node.
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

// Slide consumes the given number of bytes.
func (n *NocopyNode) Slide(delta int) bool {
	switch b := n.block.(type) {
	case []byte:
		if delta < 0 || delta > len(b) {
			return false
		}
		n.block = b[delta:]
		return true
	case *Bytes:
		return b.Slide(delta)
	case *Writer:
		return b.Slide(delta)
	default:
		return false
	}
}

// Release releases the node.
func (n *NocopyNode) Release() {
	switch b := n.block.(type) {
	case []byte:
		// ignore
	case *Bytes:
		b.Release()
	case *Writer:
		b.Release()
	}

	n.next = nil
	n.block = nil
}
