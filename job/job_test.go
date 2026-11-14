package job

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testHandler struct{}

func (testHandler) Execute(context.Context, *Request) (*Result, error) {
	return &Result{}, nil
}

func TestValidateDefinitionAppliesDefaults(t *testing.T) {
	definition := &Definition{
		ID:           "daily",
		Type:         "daily_mail",
		ScheduleType: ScheduleOnce,
	}

	if err := ValidateDefinition(definition); err != nil {
		t.Fatalf("ValidateDefinition() error = %v", err)
	}
	if definition.MaxAttempts != 1 {
		t.Fatalf("MaxAttempts = %d, want 1", definition.MaxAttempts)
	}
	if definition.MisfirePolicy != MisfireFireOnce {
		t.Fatalf("MisfirePolicy = %q, want %q", definition.MisfirePolicy, MisfireFireOnce)
	}
	if definition.ConcurrencyPolicy != ForbidConcurrent {
		t.Fatalf("ConcurrencyPolicy = %q, want %q", definition.ConcurrencyPolicy, ForbidConcurrent)
	}
}

func TestClassifyError(t *testing.T) {
	base := errors.New("temporary")
	tests := []struct {
		name string
		err  error
		want ErrorClass
	}{
		{name: "retryable", err: NewRetryableError(base), want: ErrorRetryable},
		{name: "canceled", err: context.Canceled, want: ErrorCanceled},
		{name: "non retryable", err: base, want: ErrorNonRetryable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ClassifyError(test.err); got != test.want {
				t.Fatalf("ClassifyError() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNextRetryAt(t *testing.T) {
	now := time.Unix(100, 0)
	policy := RetryPolicy{
		Initial:    time.Second,
		MaxDelay:   10 * time.Second,
		Multiplier: 2,
	}

	if got := nextRetryAt(1, policy, now); !got.Equal(now.Add(time.Second)) {
		t.Fatalf("attempt 1 retry time = %v", got)
	}
	if got := nextRetryAt(3, policy, now); !got.Equal(now.Add(4 * time.Second)) {
		t.Fatalf("attempt 3 retry time = %v", got)
	}
	if got := nextRetryAt(8, policy, now); !got.Equal(now.Add(10 * time.Second)) {
		t.Fatalf("capped retry time = %v", got)
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("test", testHandler{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if _, ok := registry.Get("test"); !ok {
		t.Fatal("Get() did not find registered handler")
	}
}
