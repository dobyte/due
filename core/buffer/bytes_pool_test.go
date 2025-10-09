package buffer_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
)

func Test_BytesPool(t *testing.T) {
	p := buffer.NewBytesPoolWithCapacity(1024)
	b := p.Get(3)
	t.Log(b.Bytes())
	p.Put(b)
}
