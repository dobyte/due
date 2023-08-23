package chain_test

import (
	"fmt"
	"github.com/dobyte/due/v2/chain"
	"testing"
)

func TestNewChain(t *testing.T) {
	c := chain.NewChain()

	defer c.FireTail()

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
