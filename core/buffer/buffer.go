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
	Mount(block any, whence ...Whence)
	// Malloc 分配一块内存给Writer
	Malloc(cap int, whence ...Whence) *Writer
	// Visit 迭代
	Visit(fn func(node *NocopyNode) bool) bool
	// Delay 设置延迟释放点
	Delay(delay int32)
	// Release 释放
	Release(force ...bool)
}
