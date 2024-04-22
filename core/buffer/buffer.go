package buffer

type Whence int

const (
	Head Whence = iota // 头部
	Tail               // 尾部
)

type Buffer interface {
	// Len 获取字节长度
	Len() int
	// Mount 挂载数据到Buffer上
	Mount(data []byte, whence ...Whence)
	// Malloc 分配一块内存给Writer
	Malloc(cap int, whence ...Whence) *Writer
	// Range 迭代
	Range(fn func(node *NocopyNode) bool)
	// Release 释放
	Release()
}
