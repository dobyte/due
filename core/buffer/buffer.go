package buffer

type Whence int

const (
	Head Whence = iota // 头部
	Tail               // 尾部
)

const (
	b8 = 1 << iota
	b16
	b32
	b64
)

type Buffer interface {
	// Len 获取字节长度
	Len() int
	// Bytes 获取所有字节（性能较低，不推荐使用）
	Bytes() []byte
	// Release 释放
	Release()
}
