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
	// Mount 挂载数据到Buffer上
	Mount(block interface{}, whence ...Whence)
	// Malloc 分配一块内存给Writer
	Malloc(cap int, whence ...Whence) *Writer
	// Range 迭代
	Range(fn func(node *NocopyNode) bool)
	// Release 释放
	Release()
}
