package redis

import (
	"context"
	"testing"
	"time"

	"github.com/dobyte/due/v2/limiter"
)

func TestLimiter_CommandContext(t *testing.T) {
	maker := NewMaker(WithTimeout(200 * time.Millisecond))
	t.Cleanup(func() { _ = maker.Close() })

	l, err := maker.Make("context", limiter.Rule{Quota: 1, Period: time.Second})
	if err != nil {
		t.Fatal(err)
	}

	ml := l.(*Limiter)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	allocs := testing.AllocsPerRun(1000, func() {
		_, ctxCancel := ml.commandContext(ctx)
		if ctxCancel != nil {
			ctxCancel()
		}
	})

	if allocs != 0 {
		t.Fatalf("expect zero allocations for a shorter caller deadline, got: %v", allocs)
	}

	if _, ctxCancel := ml.commandContext(context.Background()); ctxCancel == nil {
		t.Fatal("expect a cancel function when the caller has no deadline")
	} else {
		ctxCancel()
	}
}

func TestLimiter_CommandContext_NoTimeout(t *testing.T) {
	maker := NewMaker(WithTimeout(0))
	t.Cleanup(func() { _ = maker.Close() })

	l, err := maker.Make("no-timeout", limiter.Rule{Quota: 1, Period: time.Second})
	if err != nil {
		t.Fatal(err)
	}

	ml := l.(*Limiter)

	allocs := testing.AllocsPerRun(1000, func() {
		_, ctxCancel := ml.commandContext(context.Background())
		if ctxCancel != nil {
			ctxCancel()
		}
	})

	if allocs != 0 {
		t.Fatalf("expect zero allocations when the decision timeout is disabled, got: %v", allocs)
	}
}
