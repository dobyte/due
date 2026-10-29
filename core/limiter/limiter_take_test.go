package limiter_test

import (
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/limiter"
)

func TestLimiter_TakeAt(t *testing.T) {
	l := limiter.NewLimiter(1, 1)
	now := time.Now()

	if _, _, allowed := l.TakeAt(now, 1); !allowed {
		t.Fatal("expect the first request to be allowed")
	}

	remaining, retryAfter, allowed := l.TakeAt(now, 1)
	if allowed {
		t.Fatal("expect the second request to be denied")
	}

	if remaining != 0 {
		t.Fatalf("expect no remaining tokens, got: %d", remaining)
	}

	if retryAfter <= 0 {
		t.Fatalf("expect a positive retry after, got: %v", retryAfter)
	}

	if _, _, allowed = l.TakeAt(now.Add(time.Second), 1); !allowed {
		t.Fatal("expect the request after refill to be allowed")
	}
}
