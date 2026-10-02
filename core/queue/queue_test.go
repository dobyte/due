package queue_test

import (
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
)

// waitSignal waits for sig and fails the test with msg when it does not arrive within a second.
func waitSignal(t *testing.T, sig <-chan struct{}, msg string) {
	t.Helper()

	select {
	case <-sig:
	case <-time.After(time.Second):
		t.Fatal(msg)
	}
}

// TestQueue_WriteAndRead verifies queue write, read and data acknowledgement.
func TestQueue_WriteAndRead(t *testing.T) {
	q := queue.NewQueue[int](4, 0)

	for i := 0; i < 4; i++ {
		if err := q.Write(i); err != nil {
			t.Fatalf("write data failed, err: %v", err)
		}
	}

	for i := 0; i < 4; i++ {
		v, ok := <-q.Read()
		if !ok {
			t.Fatal("the queue channel is closed unexpectedly")
		}

		if v != i {
			t.Fatalf("the data is out of order, expect: %d, actual: %d", i, v)
		}

		q.Done(false)
	}

	q.Close()

	sig := make(chan struct{})
	go func() {
		q.Wait()
		close(sig)
	}()

	waitSignal(t, sig, "wait did not return after the queue is closed")
}

// TestQueue_WriteWithExternalLock verifies write and close when reusing the caller's read-write lock.
func TestQueue_WriteWithExternalLock(t *testing.T) {
	q := queue.NewQueue[int](2, 0, &sync.RWMutex{})

	if err := q.Write(1); err != nil {
		t.Fatalf("write data failed, err: %v", err)
	}

	v, ok := <-q.Read()
	if !ok {
		t.Fatal("the queue channel is closed unexpectedly")
	}

	if v != 1 {
		t.Fatalf("the data is out of order, expect: 1, actual: %d", v)
	}

	q.Done(false)
	q.Close()
}

// TestQueue_WriteTimeout verifies the timeout behavior of a non-blocking write when the queue is full.
func TestQueue_WriteTimeout(t *testing.T) {
	q := queue.NewQueue[int](1, 10*time.Millisecond)

	if err := q.Write(1); err != nil {
		t.Fatalf("write data failed, err: %v", err)
	}

	if err := q.Write(2); !errors.Is(err, errors.ErrWriteTimeout) {
		t.Fatalf("write data expect ErrWriteTimeout, actual: %v", err)
	}

	q.Close()
}

// TestQueue_WriteBlock verifies that a blocking write ignores the timeout and succeeds once the consumer frees space.
func TestQueue_WriteBlock(t *testing.T) {
	q := queue.NewQueue[int](1, 10*time.Millisecond)

	if err := q.Write(1); err != nil {
		t.Fatalf("write data failed, err: %v", err)
	}

	sig := make(chan struct{})

	go func() {
		if err := q.Write(2, true); err != nil {
			t.Errorf("write data failed, err: %v", err)
		}

		close(sig)
	}()

	select {
	case <-sig:
		t.Fatal("the blocking write did not wait for the free space")
	case <-time.After(50 * time.Millisecond):
	}

	if v, _ := <-q.Read(); v != 1 {
		t.Fatalf("the data is out of order, expect: 1, actual: %d", v)
	}

	q.Done(false)

	waitSignal(t, sig, "the blocking write did not return after the free space")

	if v, _ := <-q.Read(); v != 2 {
		t.Fatalf("the data is out of order, expect: 2, actual: %d", v)
	}

	q.Done(false)
	q.Close()
}

// TestQueue_Hang verifies that the queue hangs and releases the wait after the end signal is consumed.
func TestQueue_Hang(t *testing.T) {
	q := queue.NewQueue[any](4, 0)

	if err := q.Write(nil); err != nil {
		t.Fatalf("write sentinel data failed, err: %v", err)
	}

	v, ok := <-q.Read()
	if !ok {
		t.Fatal("the queue channel is closed unexpectedly")
	}

	q.Done(v == nil)

	sig := make(chan struct{})
	go func() {
		q.Wait()
		close(sig)
	}()

	waitSignal(t, sig, "wait did not return after the sentinel data is handled")

	if err := q.Write(1); !errors.Is(err, errors.ErrQueueHanged) {
		t.Fatalf("write data expect ErrQueueHanged, actual: %v", err)
	}

	q.Close()
}

// TestQueue_Closed verifies write rejection, repeated close and residual data cleaning after the queue is closed.
func TestQueue_Closed(t *testing.T) {
	q := queue.NewQueue[int](2, 0)

	if err := q.Write(1); err != nil {
		t.Fatalf("write data failed, err: %v", err)
	}

	q.Close()
	q.Close()

	if err := q.Write(2); !errors.Is(err, errors.ErrQueueClosed) {
		t.Fatalf("write data expect ErrQueueClosed, actual: %v", err)
	}

	var count int

	q.Clean(func(int) { count++ })

	if count != 1 {
		t.Fatalf("clean residual data failed, expect: 1, actual: %d", count)
	}
}

// TestTasker_CommitAndHandle verifies task commit, handling and wait group release.
func TestTasker_CommitAndHandle(t *testing.T) {
	tk := queue.NewTasker(4, 0)

	executed := make(chan struct{}, 2)

	wg, err := tk.Commit(func() { executed <- struct{}{} }, true)
	if err != nil {
		t.Fatalf("commit task failed, err: %v", err)
	}

	if wg == nil {
		t.Fatal("the wait group is nil in the waiting mode")
	}

	tk.Handle(<-tk.Read(), true)

	waitSignal(t, executed, "the task is not executed")

	sig := make(chan struct{})
	go func() {
		wg.Wait()
		close(sig)
	}()

	waitSignal(t, sig, "the wait group is not released after the task is handled")

	wg2, err := tk.Commit(func() { executed <- struct{}{} })
	if err != nil {
		t.Fatalf("commit task failed, err: %v", err)
	}

	if wg2 != nil {
		t.Fatal("the wait group is not nil in the non-waiting mode")
	}

	tk.Handle(<-tk.Read(), true)

	waitSignal(t, executed, "the task is not executed")

	tk.Close()
}

// TestTasker_Done verifies that the end signal hangs the queue and rejects subsequent task commits.
func TestTasker_Done(t *testing.T) {
	tk := queue.NewTasker(4, 0)

	if err := tk.Done(); err != nil {
		t.Fatalf("done failed, err: %v", err)
	}

	tk.Handle(<-tk.Read(), true)

	sig := make(chan struct{})
	go func() {
		tk.Wait()
		close(sig)
	}()

	waitSignal(t, sig, "wait did not return after the sentinel task is handled")

	if _, err := tk.Commit(func() {}); !errors.Is(err, errors.ErrQueueHanged) {
		t.Fatalf("commit task expect ErrQueueHanged, actual: %v", err)
	}

	tk.Close()
}

// TestTasker_Clean verifies that the wait group is released after unfinished tasks are cleaned.
func TestTasker_Clean(t *testing.T) {
	tk := queue.NewTasker(4, 0)

	wg, err := tk.Commit(func() { t.Error("the residual task should not be executed") }, true)
	if err != nil {
		t.Fatalf("commit task failed, err: %v", err)
	}

	if wg == nil {
		t.Fatal("the wait group is nil in the waiting mode")
	}

	tk.Close()
	tk.Clean()

	sig := make(chan struct{})
	go func() {
		wg.Wait()
		close(sig)
	}()

	waitSignal(t, sig, "the wait group is not released after the task is cleaned")
}
