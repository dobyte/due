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

// AddToHead 添加头部
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

// AddToTail 添加到尾部
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

// FireHead 从头部开始执行
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
}

// FireTail 从尾部开始执行
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
}

// Cancel 取消调用栈
func (c *Chain) Cancel() {
	c.canceled = true
}

// Recover 恢复调用栈
func (c *Chain) Recover() {
	c.canceled = false
}

// Release 释放调用栈
func (c *Chain) Release() {
	c.canceled = true
	c.head = nil
	c.tail = nil
}
