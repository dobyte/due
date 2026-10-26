package chains

import (
	"sync"

	"github.com/dobyte/due/v2/utils/xcall"
)

type Chain struct {
	head     *node
	tail     *node
	canceled bool
}

type node struct {
	prev *node
	next *node
	fn   func()
}

// nodePool recycles the link nodes allocated by a chain.
var nodePool = sync.Pool{
	New: func() any { return &node{} },
}

func NewChain() *Chain {
	return &Chain{}
}

// AddToHead adds fn to the head of the chain.
func (c *Chain) AddToHead(fn func()) {
	if c.head == nil || c.canceled {
		c.releaseNodes()

		head := nodePool.Get().(*node)
		head.fn = fn
		head.prev = nil
		head.next = nil
		c.head = head
		c.tail = head
	} else {
		head := nodePool.Get().(*node)
		head.fn = fn
		head.prev = nil
		head.next = c.head
		c.head.prev = head
		c.head = head
	}
}

// AddToTail adds fn to the tail of the chain.
func (c *Chain) AddToTail(fn func()) {
	if c.tail == nil || c.canceled {
		c.releaseNodes()

		tail := nodePool.Get().(*node)
		tail.fn = fn
		tail.prev = nil
		tail.next = nil
		c.tail = tail
		c.head = tail
	} else {
		tail := nodePool.Get().(*node)
		tail.fn = fn
		tail.prev = c.tail
		tail.next = nil
		c.tail.next = tail
		c.tail = tail
	}
}

// FireHead executes the chain from the head.
func (c *Chain) FireHead() {
	if c.canceled {
		return
	}

	// Detach the chain first so that a callback releasing or cancelling the chain cannot recycle
	// a node that is still pending in this loop.
	head := c.head
	c.head = nil
	c.tail = nil

	for head != nil {
		next := head.next
		fn := head.fn
		head.prev = nil
		head.next = nil
		head.fn = nil
		nodePool.Put(head)
		xcall.Call(fn)
		head = next
	}

	c.head = nil
	c.tail = nil
	c.canceled = false
}

// FireTail executes the chain from the tail.
func (c *Chain) FireTail() {
	if c.canceled {
		return
	}

	// Detach the chain first so that a callback releasing or cancelling the chain cannot recycle
	// a node that is still pending in this loop.
	tail := c.tail
	c.head = nil
	c.tail = nil

	for tail != nil {
		prev := tail.prev
		fn := tail.fn
		tail.prev = nil
		tail.next = nil
		tail.fn = nil
		nodePool.Put(tail)
		xcall.Call(fn)
		tail = prev
	}

	c.head = nil
	c.tail = nil
	c.canceled = false
}

// Cancel cancels execution of the chain.
func (c *Chain) Cancel() {
	c.canceled = true
}

// Recover restores the chain after it has been canceled.
func (c *Chain) Recover() {
	c.canceled = false
}

// Release releases the chain.
func (c *Chain) Release() {
	c.releaseNodes()
	c.head = nil
	c.tail = nil
	c.canceled = false
}

// releaseNodes returns every link node to the pool without touching the chain state.
func (c *Chain) releaseNodes() {
	for head := c.head; head != nil; {
		next := head.next
		head.prev = nil
		head.next = nil
		head.fn = nil
		nodePool.Put(head)
		head = next
	}
}
