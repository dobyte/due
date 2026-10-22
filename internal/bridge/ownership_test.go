package bridge

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
)

type countedBuffer struct {
	buffer.Buffer

	releases int
}

func (b *countedBuffer) Release() {
	b.releases++
	b.Buffer.Release()
}

func TestCopyBufferOwnership(t *testing.T) {
	for _, size := range []int{0, 64, 256, 1024, 16 << 10, 64 << 10, 64<<10 + 1, 128 << 10} {
		for _, shape := range []string{"flat", "fragmented", "sliced"} {
			t.Run(fmt.Sprintf("bytes=%d/%s", size, shape), func(t *testing.T) {
				payload := bytes.Repeat([]byte("x"), size)
				var src buffer.Buffer = buffer.NewBytes(payload, true)
				if shape != "flat" {
					src = buffer.NewNocopyBuffer(payload[:size/2],
						buffer.NewNocopyBuffer(payload[size/2:]))
				}
				if shape == "sliced" && size > 0 {
					src.Slide(1)
				}
				want := bytes.Clone(src.Bytes())
				tracked := &countedBuffer{Buffer: src}
				dst := CopyBuffer(tracked)
				defer dst.Release()
				clear(payload)
				if tracked.releases != 1 {
					t.Fatalf("source released %d times", tracked.releases)
				}
				if dst.Len() != len(want) || !bytes.Equal(dst.Bytes(), want) {
					t.Fatal("copy shares storage or has an incorrect readable range")
				}
			})
		}
	}
}

func TestFullQueueDeadlinePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		caller time.Duration
		call   time.Duration
		write  time.Duration
		ack    bool
		want   error
	}{
		{name: "write", write: time.Second, want: errors.ErrWriteTimeout},
		{name: "caller", caller: time.Second, write: 2 * time.Second, want: context.DeadlineExceeded},
		{name: "call", call: time.Second, write: 2 * time.Second, ack: true, want: context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := NewDirectory().Register("node", nil, 1, 1)
				defer e.Close()
				started := make(chan struct{})
				_, err := e.Submit(context.Background(), 0, 0, false, func(ctx context.Context) (any, error) {
					close(started)
					<-ctx.Done()
					return nil, nil
				}, nil)
				if err != nil {
					t.Fatal(err)
				}
				<-started
				run := func(context.Context) (any, error) {
					t.Error("rejected or shutdown command executed")
					return nil, nil
				}
				if _, err := e.Submit(context.Background(), 0, 0, false, run, nil); err != nil {
					t.Fatal(err)
				}
				ctx := context.Background()
				if tc.caller > 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, tc.caller)
					defer cancel()
				}
				releases := 0
				_, err = e.Submit(ctx, 0, tc.call, tc.ack, run, func() { releases++ }, tc.write)
				if !errors.Is(err, tc.want) || releases != 1 {
					t.Fatalf("error=%v releases=%d, want %v and one release", err, releases, tc.want)
				}
			})
		})
	}
}

func TestAdmissionPreservesShardOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := NewDirectory().Register("node", nil, 1, 1)
		defer e.Close()
		resume := make(chan struct{})
		started := make(chan struct{})
		_, err := e.Submit(context.Background(), 0, 0, false, func(context.Context) (any, error) {
			close(started)
			<-resume
			return nil, nil
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		<-started
		var order []int
		_, err = e.Submit(context.Background(), 0, time.Second, false, func(context.Context) (any, error) {
			order = append(order, 1)
			return nil, nil
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			_, err := e.Submit(context.Background(), 0, time.Second, true, func(context.Context) (any, error) {
				order = append(order, 2)
				return nil, nil
			}, nil)
			done <- err
		}()
		synctest.Wait()
		close(resume)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if len(order) != 2 || order[0] != 1 || order[1] != 2 {
			t.Fatalf("execution order: %v", order)
		}
	})
}

func TestTimedOutCallRetainsCopiedBuffer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := NewDirectory().Register("node", nil, 1, 1)
		defer e.Close()
		payload := bytes.Repeat([]byte("x"), 256)
		copied := CopyBuffer(buffer.NewBytes(payload, true))
		resume := make(chan struct{})
		observed := make(chan []byte, 1)
		_, err := e.Submit(context.Background(), 0, time.Second, true, func(ctx context.Context) (any, error) {
			select {
			case <-resume:
				observed <- bytes.Clone(copied.Bytes())
			case <-ctx.Done():
			}
			copied.Release()
			return nil, nil
		}, copied.Release)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("call timeout: %v", err)
		}
		other := CopyBuffer(buffer.NewBytes(payload, true))
		clear(other.Bytes())
		other.Release()
		close(resume)
		if got := <-observed; !bytes.Equal(got, payload) {
			t.Fatal("timed-out call's buffer was recycled before execution completed")
		}
	})
}
