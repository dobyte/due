package chains

import "github.com/dobyte/due/v2/utils/xcall"

type Chain struct {
	head *node
	tail *node
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
	if c.head == nil {
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
	if c.tail == nil {
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
	c.head = nil
	c.tail = nil
}
