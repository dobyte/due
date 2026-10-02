package chains

import (
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

func NewChain() *Chain {
	return &Chain{}
}

// AddToHead adds fn to the head of the chain.
func (c *Chain) AddToHead(fn func()) {
	if c.head == nil || c.canceled {
		c.head = &node{fn: fn}
		c.tail = c.head
	} else {
		head := &node{fn: fn, next: c.head}
		c.head.prev = head
		c.head = head
	}
}

// AddToTail adds fn to the tail of the chain.
func (c *Chain) AddToTail(fn func()) {
	if c.tail == nil || c.canceled {
		c.tail = &node{fn: fn}
		c.head = c.tail
	} else {
		tail := &node{fn: fn, prev: c.tail}
		c.tail.next = tail
		c.tail = tail
	}
}

// FireHead executes the chain from the head.
func (c *Chain) FireHead() {
	if c.canceled {
		return
	}

	for head := c.head; head != nil; {
		xcall.Call(head.fn)
		next := head.next
		head.prev = nil
		head.next = nil
		head.fn = nil
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

	for tail := c.tail; tail != nil; {
		xcall.Call(tail.fn)
		prev := tail.prev
		tail.prev = nil
		tail.next = nil
		tail.fn = nil
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
	c.head = nil
	c.tail = nil
	c.canceled = false
}
