package job

import "time"

// nextRetryAt calculates an exponential retry time.
func nextRetryAt(attempt int, policy RetryPolicy, now time.Time) time.Time {
	if policy.Initial <= 0 {
		policy.Initial = time.Second
	}
	if policy.MaxDelay <= 0 {
		policy.MaxDelay = time.Minute
	}
	if policy.Multiplier < 1 {
		policy.Multiplier = 2
	}

	delay := policy.Initial
	for i := 1; i < attempt; i++ {
		if delay >= policy.MaxDelay {
			delay = policy.MaxDelay
			break
		}
		delay = time.Duration(float64(delay) * policy.Multiplier)
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
	}

	return now.Add(delay)
}

// runTimeout returns a run-specific timeout, falling back to the lease duration.
func runTimeout(r *Run, leaseDuration time.Duration) time.Duration {
	if r == nil || r.Timeout <= 0 {
		return leaseDuration
	}

	return r.Timeout
}
