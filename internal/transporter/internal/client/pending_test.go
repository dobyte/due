package client

import (
	"sync"
	"sync/atomic"
	"testing"
)

func BenchmarkPending(b *testing.B) {
	var (
		sequence uint64
		call     = make(chan []byte)
		ch       = make(chan uint64, 10240)
		p        = newPending()
		wg       sync.WaitGroup
	)

	b.ResetTimer()

	wg.Add(b.N)

	go func() {
		for i := 0; i < b.N; i++ {
			seq := atomic.AddUint64(&sequence, 1)

			p.store(seq, call)

			ch <- seq
		}
	}()

	go func() {
		for {
			select {
			case seq, ok := <-ch:
				if !ok {
					return
				}

				p.extract(seq)

				wg.Done()
			}
		}
	}()

	wg.Wait()

	close(ch)
}

func BenchmarkSyncMap(b *testing.B) {
	var (
		sequence uint64
		call     = make(chan []byte)
		ch       = make(chan uint64, 10240)
		m        sync.Map
		wg       sync.WaitGroup
	)

	b.ResetTimer()

	wg.Add(b.N)

	go func() {
		for i := 0; i < b.N; i++ {
			seq := atomic.AddUint64(&sequence, 1)

			m.Store(seq, call)

			ch <- seq
		}
	}()

	go func() {
		for {
			select {
			case seq, ok := <-ch:
				if !ok {
					return
				}

				m.Load(seq)

				m.Delete(seq)

				wg.Done()
			}
		}
	}()

	wg.Wait()

	close(ch)
}
