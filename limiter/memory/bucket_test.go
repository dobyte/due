package memory

import (
	"testing"
	"time"
)

// TestTokenBucketTakeAt verifies the refill, denial and retry-after behavior of the token bucket.
func TestTokenBucketTakeAt(t *testing.T) {
	b := newTokenBucket(1, 1)
	now := time.Now()

	if _, _, allowed := b.takeAt(now, 1); !allowed {
		t.Fatal("expect the first request to be allowed")
	}

	remaining, retryAfter, allowed := b.takeAt(now, 1)
	if allowed {
		t.Fatal("expect the second request to be denied")
	}

	if remaining != 0 {
		t.Fatalf("expect no remaining tokens, got: %d", remaining)
	}

	if retryAfter <= 0 {
		t.Fatalf("expect a positive retry after, got: %v", retryAfter)
	}

	if _, _, allowed = b.takeAt(now.Add(time.Second), 1); !allowed {
		t.Fatal("expect the request after refill to be allowed")
	}
}
