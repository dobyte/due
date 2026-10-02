package buffer

// Whence specifies where a node is mounted.
type Whence int

const (
	// Head mounts the node at the head.
	Head Whence = iota
	// Tail mounts the node at the tail.
	Tail
)

const (
	b8 = 1 << iota
	b16
	b32
	b64
)

type Buffer interface {
	// Len returns the byte length.
	Len() int
	// Nodes returns the number of nodes.
	Nodes() int
	// Delay sets the delayed release point.
	Delay(delay int)
	// Bytes returns all bytes. It is relatively slow and not recommended.
	Bytes() []byte
	// Release releases the buffer.
	Release()
	// Slide slides the lower index.
	Slide(lower int) bool
	// VisitBytes iterates over all bytes.
	VisitBytes(fn func(bytes []byte) bool) bool
}
