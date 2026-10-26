package chains_test

import (
	"fmt"
	"github.com/dobyte/due/v2/core/chains"
	"testing"
)

func TestNewChain(t *testing.T) {
	c := chains.NewChain()

	defer c.FireHead()

	c.AddToHead(func() {
		fmt.Println(1111)
	})

	c.AddToHead(func() {
		fmt.Println(2222)
	})

	c.AddToHead(func() {
		fmt.Println(3333)
	})
}

func TestChainReuse(t *testing.T) {
	c := chains.NewChain()

	for i := range 3 {
		var head, tail []int

		c.AddToHead(func() { head = append(head, i) })
		c.AddToTail(func() { tail = append(tail, i) })
		c.FireHead()

		if len(head) != 1 || head[0] != i {
			t.Fatalf("round %d: unexpected head order: %v", i, head)
		}
		if len(tail) != 1 || tail[0] != i {
			t.Fatalf("round %d: unexpected tail order: %v", i, tail)
		}
	}

	var tailOrder []int

	c.AddToTail(func() { tailOrder = append(tailOrder, 1) })
	c.AddToTail(func() { tailOrder = append(tailOrder, 2) })
	c.FireTail()

	if len(tailOrder) != 2 || tailOrder[0] != 2 || tailOrder[1] != 1 {
		t.Fatalf("unexpected tail-first order: %v", tailOrder)
	}

	c.Release()

	reused := false

	c.AddToHead(func() { reused = true })
	c.FireHead()

	if !reused {
		t.Fatal("released chain should be reusable")
	}
}
