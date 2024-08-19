package node

type base struct {
	fnChan chan func() // 调用函数
}

// Invoke 调用函数（Actor内线程安全）
func (b *base) Invoke(fn func()) {
	b.fnChan <- fn
}
