package xcall_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xcall"
)

func TestBackoff(t *testing.T) {
	err := xcall.Backoff(context.Background(), func(ctx context.Context, attempt int) (bool, error) {
		fmt.Printf("attempt: %d\n", attempt)

		return true, errors.New("backoff test error")
	}, 5, 100*time.Millisecond, 1000*time.Millisecond)

	t.Logf("err: %v", err)
}

func TestGo(t *testing.T) {
	done := make(chan struct{})

	xcall.Go(func() {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("xcall.Go did not execute func()")
	}
}

func TestGoFuncError(t *testing.T) {
	done := make(chan struct{})

	xcall.Go(func() error {
		close(done)
		return errors.New("test error")
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("xcall.Go did not execute func() error")
	}
}

func TestGoPanic(t *testing.T) {
	entered := make(chan struct{})

	xcall.Go(func() {
		close(entered)
		panic("test panic")
	})

	<-entered

	// panic 应被 Call 捕获而不致进程退出；此处等待 goroutine 完成 recover
	time.Sleep(50 * time.Millisecond)
}
