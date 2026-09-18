package buffer

// Whence 指定节点挂载位置
type Whence int

const (
	// Head 头部
	Head Whence = iota
	// Tail 尾部
	Tail
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
	// Nodes 获取节点数
	Nodes() int
	// Delay 设置延迟释放点
	Delay(delay int)
	// Bytes 获取所有字节（性能较低，不推荐使用）
	Bytes() []byte
	// Release 释放
	Release()
	// MoveTo 移动游标到指定位置
	MoveTo(pos int) bool
	// VisitBytes 迭代所有字节
	VisitBytes(fn func(bytes []byte) bool) bool
}
