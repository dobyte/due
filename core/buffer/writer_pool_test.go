package buffer_test

import (
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
)

func TestWriterPool_Get(t *testing.T) {
	for range 100 {
		writer := buffer.MallocWriter(9)
		writer.Release()
	}
}

func BenchmarkWriterPool_Get(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		writer := buffer.MallocWriter(9)
		writer.Release()
	}
}
