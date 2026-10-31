package memory

import (
	"sync"
	"time"
)

// tokenBucket is the in-process token bucket of one rate limiting key.
//
// It implements the same refill semantics as the redis backend script and is internal to this
// package: the public limiting API is [Maker] and [Limiter].
type tokenBucket struct {
	mu           sync.Mutex
	cap          float64
	num          float64
	rate         float64
	lastFillTime time.Time
}

// newTokenBucket creates a full bucket with the given capacity and refill rate in tokens per second.
func newTokenBucket(cap, rate float64) *tokenBucket {
	return &tokenBucket{
		cap:          cap,
		num:          cap,
		rate:         rate,
		lastFillTime: time.Now(),
	}
}

// takeAt takes n tokens from the bucket using now as the current time and reports the remaining
// tokens and the decision.
//
// now should come from time.Now so that its monotonic reading is preserved. The remaining value is
// truncated to an integer. When the request is denied, retryAfter reports how long it takes until n
// tokens are available; it is zero when the request is allowed or when the bucket cannot refill
// because the rate is not positive.
func (b *tokenBucket) takeAt(now time.Time, n int) (remaining int, retryAfter time.Duration, allowed bool) {
	b.mu.Lock()

	num := now.Sub(b.lastFillTime).Seconds() * b.rate

	if num > 0 {
		b.num = min(b.num+num, b.cap)
		b.lastFillTime = now
	}

	if b.num >= float64(n) {
		b.num -= float64(n)
		remaining, allowed = int(b.num), true
		b.mu.Unlock()
		return remaining, 0, allowed
	}

	if b.rate > 0 {
		retryAfter = time.Duration((float64(n) - b.num) / b.rate * float64(time.Second))
		if retryAfter < time.Millisecond {
			retryAfter = time.Millisecond
		}
	}

	remaining = int(b.num)
	b.mu.Unlock()
	return remaining, retryAfter, false
}
