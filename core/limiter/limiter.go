package limiter

import (
	"sync"
	"time"
)

// Limiter 令牌桶限流器实现
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

func (l *Limiter) Allow(n ...int) bool {
	if len(n) > 0 && n[0] > 0 {
		return l.doAllow(n[0])
	} else {
		return l.doAllow(1)
	}
}

func (l *Limiter) doAllow(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	num := now.Sub(l.lastFillTime).Seconds() * l.rate

	if num > 0 {
		l.num = min(l.num+num, l.cap)
		l.lastFillTime = now
	}

	if l.num >= float64(n) {
		l.num -= float64(n)
		return true
	}

	return false
}
