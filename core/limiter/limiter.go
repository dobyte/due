package limiter

import (
	"github.com/dobyte/due/v2/utils/xtime"
	"sync"
)

// Limiter 令牌桶限流器实现
type Limiter struct {
	mu           sync.Mutex
	cap          float64
	num          float64
	rate         float64
	lastFillTime xtime.Time
}

func NewLimiter(cap, rate float64) *Limiter {
	return &Limiter{
		cap:          cap,
		num:          cap,
		rate:         rate,
		lastFillTime: xtime.Now(),
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

	now := xtime.Now()
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
