package node

import "time"

type Timer struct {
	node  *Node
	timer *time.Timer
}

// Stop 停止定时器
func (t *Timer) Stop() (ok bool) {
	defer func() {
		if ok && t.node != nil {
			t.node.doneWait()
		}
	}()

	if t == nil {
		return
	}

	ok = t.timer.Stop()

	return
}
