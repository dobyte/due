package bridge

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"go.uber.org/goleak"
)

// Ignore framework logging workers already present before any bridge is created.
func TestMain(m *testing.M) { goleak.VerifyTestMain(m, goleak.IgnoreCurrent()) }

func TestCloseUnblocksFullQueueAndReleasesPending(t *testing.T) {
	d := NewDirectory()
	e := d.Register("node", nil, 1, 1)
	defer e.Close()
	started := make(chan struct{})
	_, err := e.Submit(context.Background(), 0, time.Second, false, func(ctx context.Context) (any, error) {
		close(started)
		<-ctx.Done()
		return nil, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	var mu sync.Mutex
	releases := 0
	release := func() { mu.Lock(); releases++; mu.Unlock() }
	run := func(context.Context) (any, error) { t.Error("queued command executed during close"); return nil, nil }
	if _, err := e.Submit(context.Background(), 0, time.Second, false, run, release); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Submit(context.Background(), 0, time.Millisecond, false, run, release); !errors.Is(err, errors.ErrWriteTimeout) {
		t.Fatalf("full queue: %v", err)
	}
	finished := make(chan error, 1)
	go func() { _, err := e.Submit(context.Background(), 0, 0, false, run, release); finished <- err }()
	e.Close()
	if err := <-finished; !errors.Is(err, errors.ErrServerClosed) {
		t.Fatalf("close: %v", err)
	}
	mu.Lock()
	got := releases
	mu.Unlock()
	if got != 3 {
		t.Fatalf("release count %d, want 3", got)
	}
	if _, err := d.Lookup(e.Token(), "node"); !errors.Is(err, errors.ErrServerClosed) {
		t.Fatalf("stale incarnation: %v", err)
	}
}

func TestAcknowledgementTimeoutDoesNotBlockWorker(t *testing.T) {
	e := NewDirectory().Register("gate", nil, 1, 1)
	defer e.Close()
	resume := make(chan struct{})
	_, err := e.Submit(context.Background(), 0, time.Millisecond, true, func(context.Context) (any, error) {
		<-resume
		return 7, nil
	}, nil)
	close(resume)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout: %v", err)
	}
	v, err := e.Submit(context.Background(), 0, time.Second, true, func(context.Context) (any, error) { return 9, nil }, nil)
	if err != nil || v != 9 {
		t.Fatalf("next call: %v, %v", v, err)
	}
}

func TestDirectoryIsolationAndCopy(t *testing.T) {
	d := NewDirectory()
	e := d.Register("node", nil, 1, 1)
	defer e.Close()
	metadata := map[string]string{"user": "value"}
	target := Target("local://node", e.Metadata(metadata)[MetadataKey])
	if metadata[MetadataKey] != "" {
		t.Fatal("user metadata mutated")
	}
	if got, _, err := d.Resolve(target, "node", false); got != e || err != nil {
		t.Fatal("local resolution", err)
	}
	if _, _, err := NewDirectory().Resolve(target, "node", false); !errors.Is(err, errors.ErrNotFoundEndpoint) {
		t.Fatal("cross-directory resolution", err)
	}
	if _, _, err := d.Resolve(target, "node", true); !errors.Is(err, errors.ErrNotFoundEndpoint) {
		t.Fatal("disabled local-only resolution", err)
	}
	src := buffer.NewBytes([]byte("payload"), true)
	src.Delay(2)
	left := CopyBuffer(src)
	right := CopyBuffer(src)
	defer left.Release()
	defer right.Release()
	left.Slide(1)
	left.Bytes()[0] = 'X'
	if string(right.Bytes()) != "payload" {
		t.Fatal("shared mutable view")
	}
}

func TestConcurrentCloseAndSubmission(t *testing.T) {
	e := NewDirectory().Register("node", nil, 4, 8)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			for j := 0; j < 100; j++ {
				_, _ = e.Submit(context.Background(), uint64(j), time.Second, true, func(context.Context) (any, error) { return nil, nil }, nil)
			}
		})
	}
	wg.Go(e.Close)
	wg.Go(e.Close)
	wg.Wait()
}
