package buffer_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
)

func Test_BytesPool(t *testing.T) {
	p := buffer.NewBytesPoolWithCapacity(1024)
	b := p.Get(20)

	t.Logf("bytes cap: %v", b.Cap())
	t.Logf("bytes len: %v", b.Len())
	t.Logf("bytes available: %v", b.Available())
	t.Logf("buffer cap: %v", cap(b.Bytes()))
	t.Logf("buffer len: %v", len(b.Bytes()))

	t.Logf("-------------------------")
	b.MoveTo(5)

	t.Logf("bytes cap: %v", b.Cap())
	t.Logf("bytes len: %v", b.Len())
	t.Logf("bytes available: %v", b.Available())
	t.Logf("buffer cap: %v", cap(b.Bytes()))
	t.Logf("buffer len: %v", len(b.Bytes()))

	b.Release()
}
