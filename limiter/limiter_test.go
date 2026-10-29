package limiter_test

import (
	"math"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/limiter"
)

func TestRule_Normalize(t *testing.T) {
	rule, err := limiter.Rule{Quota: 10}.Normalize()
	if err != nil {
		t.Fatal(err)
	}

	if rule.Period != time.Second {
		t.Fatalf("expect default period 1s, got: %v", rule.Period)
	}

	if rule.Burst != 10 {
		t.Fatalf("expect default burst 10, got: %d", rule.Burst)
	}

	if rate := rule.Rate(); rate != 10 {
		t.Fatalf("expect rate 10, got: %v", rate)
	}

	if ttl := rule.TTL(); ttl < time.Second {
		t.Fatalf("expect ttl at least 1s, got: %v", ttl)
	}
}

func TestRule_NormalizeInvalidQuota(t *testing.T) {
	for _, quota := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := (limiter.Rule{Quota: quota}).Normalize(); !errors.Is(err, errors.ErrInvalidLimiterRule) {
			t.Fatalf("quota %v: expect ErrInvalidLimiterRule, got: %v", quota, err)
		}
	}
}
