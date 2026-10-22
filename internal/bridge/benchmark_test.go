package bridge

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
)

func BenchmarkSubmit(b *testing.B) {
	for _, ack := range []bool{false, true} {
		for _, writeTimeout := range []time.Duration{0, time.Second} {
			for _, parallel := range []bool{false, true} {
				name := fmt.Sprintf("ack=%v/write=%s/parallel=%v", ack, writeTimeout, parallel)
				b.Run(name, func(b *testing.B) {
					e := NewDirectory().Register("node", nil, 4, 4096)
					b.Cleanup(e.Close)
					ctx := context.Background()
					run := func(context.Context) (any, error) { return nil, nil }
					submit := func(key uint64) {
						if _, err := e.Submit(ctx, key, time.Second, ack, run, nil, writeTimeout); err != nil {
							b.Error(err)
						}
					}
					b.ReportAllocs()
					b.ResetTimer()
					if parallel {
						var next atomic.Uint64
						b.RunParallel(func(pb *testing.PB) {
							key := next.Add(1)
							for pb.Next() {
								submit(key)
							}
						})
					} else {
						for i := 0; i < b.N; i++ {
							submit(uint64(i))
						}
					}
					// Include consumption of all accepted commands, including oneway calls.
					for key := uint64(0); key < 4; key++ {
						if _, err := e.Submit(ctx, key, time.Second, true, run, nil); err != nil {
							b.Fatal(err)
						}
					}
					b.StopTimer()
				})
			}
		}
	}
}

func BenchmarkCopyBuffer(b *testing.B) {
	for _, size := range []int{0, 64, 256, 1024, 16 << 10, 128 << 10} {
		b.Run(fmt.Sprintf("bytes=%d", size), func(b *testing.B) {
			payload := make([]byte, size)
			// A static source isolates the copy cost without allocating input per call.
			src := buffer.NewBytes(payload, true)
			b.ReportAllocs()
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				CopyBuffer(src).Release()
			}
		})
	}
}
