package buffer

type Bytes struct {
	buf []byte
	off int
}

func NewBytes(cap int) *Bytes {
	return &Bytes{buf: make([]byte, cap), off: cap}
}

// Len 返回数据长度
func (b *Bytes) Len() int {
	return b.off
}

// Cap 返回容量
func (b *Bytes) Cap() int {
	return cap(b.buf)
}

// Available 返回可用空间
func (b *Bytes) Available() int {
	return cap(b.buf) - b.off
}

// Bytes 获取字节数据
func (b *Bytes) Bytes() []byte {
	return b.buf[:b.off]
}
