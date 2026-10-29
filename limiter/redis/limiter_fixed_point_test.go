package redis

import (
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/limiter"
)

func TestRateToNanoPerMs(t *testing.T) {
	cases := []struct {
		rate float64
		want int64
	}{
		{rate: 10, want: 10_000_000},
		{rate: 1.0 / 60, want: 16_667},
		{rate: 1.0 / 86400, want: 12},
		{rate: 0, want: 1},
	}

	for _, c := range cases {
		if got := rateToNanoPerMs(c.rate); got != c.want {
			t.Fatalf("rateToNanoPerMs(%v) = %d, want %d", c.rate, got, c.want)
		}
	}
}

func TestMaker_RejectsBurstOverLuaExactInteger(t *testing.T) {
	maker := NewMaker()
	t.Cleanup(func() { _ = maker.Close() })

	burst := int(maxExactLuaInteger/tokenScale) + 1
	_, err := maker.Make("too-large", limiter.Rule{
		Quota:  1,
		Period: time.Second,
		Burst:  burst,
	})
	if err != errors.ErrInvalidLimiterRule {
		t.Fatalf("expect ErrInvalidLimiterRule, got: %v", err)
	}
}
