package limiter

import (
	"sync"
	"time"
)

// Limiter is a token-bucket rate limiter.
type Limiter struct {
	mu           sync.Mutex
	cap          float64
	num          float64
	rate         float64
	lastFillTime time.Time
}

func NewLimiter(cap, rate float64) *Limiter {
	return &Limiter{
		cap:          cap,
		num:          cap,
		rate:         rate,
		lastFillTime: time.Now(),
	}
}

// Allow reports whether n tokens can be taken from the bucket. n defaults to 1.
func (l *Limiter) Allow(n ...int) bool {
	cost := 1
	if len(n) > 0 && n[0] > 0 {
		cost = n[0]
	}

	_, _, allowed := l.TakeAt(time.Now(), cost)
	return allowed
}

// Take takes n tokens from the bucket and reports the remaining tokens and the decision.
//
// It is equivalent to TakeAt(time.Now(), n).
func (l *Limiter) Take(n int) (remaining int, retryAfter time.Duration, allowed bool) {
	return l.TakeAt(time.Now(), n)
}

// TakeAt takes n tokens from the bucket using now as the current time and reports the remaining
// tokens and the decision.
//
// It is like Take but lets callers that also need the timestamp reuse one clock read. now should
// come from time.Now so that its monotonic reading is preserved.
//
// The remaining value is truncated to an integer. When the request is denied, retryAfter reports
// how long it takes until n tokens are available; it is zero when the request is allowed or when
// the bucket cannot refill because the rate is not positive.
func (l *Limiter) TakeAt(now time.Time, n int) (remaining int, retryAfter time.Duration, allowed bool) {
	l.mu.Lock()

	num := now.Sub(l.lastFillTime).Seconds() * l.rate

	if num > 0 {
		l.num = min(l.num+num, l.cap)
		l.lastFillTime = now
	}

	if l.num >= float64(n) {
		l.num -= float64(n)
		remaining, allowed = int(l.num), true
		l.mu.Unlock()
		return remaining, 0, allowed
	}

	if l.rate > 0 {
		retryAfter = time.Duration((float64(n) - l.num) / l.rate * float64(time.Second))
		if retryAfter < time.Millisecond {
			retryAfter = time.Millisecond
		}
	}

	remaining = int(l.num)
	l.mu.Unlock()
	return remaining, retryAfter, false
}
