package node

import "time"

// Timer is a timer.
type Timer struct {
	node  *Node
	timer *time.Timer
}

// Stop stops the timer.
//
// When the timer is stopped successfully and the node is not nil, it decrements the node's wait
// count.
func (t *Timer) Stop() (ok bool) {
	if t == nil {
		return
	}

	if ok = t.timer.Stop(); ok && t.node != nil {
		t.node.doDoneWait()
	}

	return
}
