package node

import "time"

// Timer 定时器
type Timer struct {
	node  *Node
	timer *time.Timer
}

// Stop 停止定时器
// 停止成功且节点非空时递减节点等待计数
// @return @1 bool 是否成功停止定时器
func (t *Timer) Stop() (ok bool) {
	if t == nil {
		return
	}

	if ok = t.timer.Stop(); ok && t.node != nil {
		t.node.doDoneWait()
	}

	return
}
