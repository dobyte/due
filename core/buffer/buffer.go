package buffer

type Buffer struct {
	buf []byte
	off int
}

func NewBuffer(buf []byte) *Buffer {
	return &Buffer{buf: buf}
}

func (b *Buffer) Len() int {
	return len(b.buf) - b.off
}

// Cap 获取缓冲区容量
func (b *Buffer) Cap() int {
	return cap(b.buf)
}

// Reset 重置缓冲区
func (b *Buffer) Reset() {
	b.off = 0
}

func (b *Buffer) Bytes() {
	//return b.buf[b.off:]
}

func (b *Buffer) WriteInt8(v int8) {
	b.WriteUint8(uint8(v))
}

func (b *Buffer) WriteUint8(v uint8) {
	if b.off < len(b.buf) {
		b.buf[b.off] = v
	} else {
		b.buf = append(b.buf, v)
	}

	b.off++
}
