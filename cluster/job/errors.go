package job

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/errors"
)

var (
	ErrDefinitionExists    = errors.New("job definition exists")
	ErrDefinitionNotFound  = errors.New("job definition not found")
	ErrRunNotFound         = errors.New("job run not found")
	ErrRunExists           = errors.New("job run exists")
	ErrHandlerNotFound     = errors.New("job handler not found")
	ErrLeaseLost           = errors.New("job lease lost")
	ErrInvalidDefinition   = errors.New("invalid job definition")
	ErrInvalidRun          = errors.New("invalid job run")
	ErrJobClosed           = errors.New("job service is closed")
	ErrJobStarted          = errors.New("job service is started")
	ErrUnsupportedStore    = errors.New("job store lacks required capabilities")
	ErrVersionConflict     = errors.New("job version conflict")
	ErrUnsupportedSchedule = errors.New("job schedule is unsupported")
	ErrInvalidSchedule     = errors.New("invalid job schedule")
)

// RetryableError marks an error as retryable.
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	if e == nil || e.Err == nil {
		return "retryable job error"
	}

	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// ClassifyError maps a handler error to a job error class.
func ClassifyError(err error) ErrorClass {
	if err == nil {
		return ""
	}

	var retryable *RetryableError
	if errors.As(err, &retryable) {
		return ErrorRetryable
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorRetryable
	}
	if errors.Is(err, context.Canceled) {
		return ErrorCanceled
	}

	return ErrorNonRetryable
}

// NewRetryableError wraps err as retryable.
func NewRetryableError(err error) error {
	if err == nil {
		return nil
	}

	return &RetryableError{Err: err}
}

// ValidateDefinition checks the required definition fields.
func ValidateDefinition(definition *Definition) error {
	if definition == nil || definition.ID == "" || definition.Type == "" {
		return ErrInvalidDefinition
	}
	if definition.ScheduleType == "" {
		return ErrInvalidDefinition
	}
	if definition.NextRunAt.IsZero() {
		definition.NextRunAt = time.Now()
	}
	if definition.MaxAttempts < 0 || definition.Timeout < 0 {
		return ErrInvalidDefinition
	}
	if definition.MaxAttempts == 0 {
		definition.MaxAttempts = 1
	}
	if definition.MisfirePolicy == "" {
		definition.MisfirePolicy = MisfireFireOnce
	}
	if definition.ConcurrencyPolicy == "" {
		definition.ConcurrencyPolicy = ForbidConcurrent
	}

	return nil
}

// DefaultRetryPolicy returns the default retry configuration.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		Initial:     time.Second,
		MaxDelay:    time.Minute,
		Multiplier:  2,
	}
}
