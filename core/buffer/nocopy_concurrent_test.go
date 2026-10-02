package buffer_test

import (
	"encoding/binary"
	"sync"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
)

// TestNocopyBufferConcurrentFanOut verifies that one message buffer can be shared by several
// consumers that encode it concurrently, which is how broadcast style fan-out works. Every
// consumer wraps the shared message into its own request buffer and reads the size, mirroring
// what protocol.EncodePushReq does.
//
// Run with -race to detect concurrent writes to the shared message.
func TestNocopyBufferConcurrentFanOut(t *testing.T) {
	const (
		consumers = 16
		headBytes = 16
	)

	message := buffer.NewNocopyBuffer([]byte("hello world"))
	message.Delay(consumers)

	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
	)

	for range consumers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-start

			writer := buffer.MallocWriter(headBytes)
			for range headBytes / 4 {
				writer.WriteUint32s(binary.BigEndian, uint32(message.Len()))
			}

			req := buffer.NewNocopyBuffer(writer, message)
			if size, want := req.Len(), headBytes+message.Len(); size != want {
				t.Errorf("unexpected request size: got %d, want %d", size, want)
			}

			req.Release()
		}()
	}

	close(start)
	wg.Wait()
}
