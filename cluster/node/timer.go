package node

import "time"

type Timer struct {
	node  *Node
	timer *time.Timer
}

// Stop 停止定时器
func (t *Timer) Stop() (ok bool) {
	if t == nil {
		return
	}

	if ok = t.timer.Stop(); ok && t.node != nil {
		t.node.doneWait()
	}

	return
}
