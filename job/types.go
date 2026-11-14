package job

import "time"

// ScheduleType identifies how a definition calculates its next run.
type ScheduleType string

const (
	ScheduleOnce  ScheduleType = "once"
	ScheduleDelay ScheduleType = "delay"
	ScheduleFixed ScheduleType = "fixed"
	ScheduleCron  ScheduleType = "cron"
)

// DefinitionStatus is the lifecycle state of a job definition.
type DefinitionStatus string

const (
	DefinitionEnabled  DefinitionStatus = "enabled"
	DefinitionPaused   DefinitionStatus = "paused"
	DefinitionDisabled DefinitionStatus = "disabled"
	DefinitionDeleted  DefinitionStatus = "deleted"
)

// RunStatus is the lifecycle state of one execution.
type RunStatus string

const (
	RunPending   RunStatus = "pending"
	RunLeased    RunStatus = "leased"
	RunRunning   RunStatus = "running"
	RunRetryWait RunStatus = "retry_wait"
	RunSucceeded RunStatus = "succeeded"
	RunCanceled  RunStatus = "canceled"
	RunDead      RunStatus = "dead"
)

// MisfirePolicy controls how missed schedules are handled.
type MisfirePolicy string

const (
	MisfireSkip       MisfirePolicy = "skip"
	MisfireFireOnce   MisfirePolicy = "fire_once"
	MisfireFireAll    MisfirePolicy = "fire_all"
	MisfireFireLatest MisfirePolicy = "fire_latest"
)

// ConcurrencyPolicy controls overlapping runs of one definition.
type ConcurrencyPolicy string

const (
	AllowConcurrent  ConcurrencyPolicy = "allow"
	ForbidConcurrent ConcurrencyPolicy = "forbid"
	QueueConcurrent  ConcurrencyPolicy = "queue"
	ReplacePrevious  ConcurrencyPolicy = "replace"
)

// ErrorClass controls whether a failed run can be retried.
type ErrorClass string

const (
	ErrorRetryable    ErrorClass = "retryable"
	ErrorNonRetryable ErrorClass = "non_retryable"
	ErrorCanceled     ErrorClass = "canceled"
	ErrorLeaseLost    ErrorClass = "lease_lost"
)

// Definition describes a long-lived job.
type Definition struct {
	ID                string
	Name              string
	Type              string
	Payload           []byte
	ScheduleType      ScheduleType
	ScheduleExpr      string
	Timezone          string
	NextRunAt         time.Time
	Status            DefinitionStatus
	MisfirePolicy     MisfirePolicy
	ConcurrencyPolicy ConcurrencyPolicy
	MaxAttempts       int
	RetryPolicy       RetryPolicy
	Timeout           time.Duration
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Run describes one execution of a definition.
type Run struct {
	ID                string
	JobID             string
	TaskType          string
	Payload           []byte
	ScheduledAt       time.Time
	ReadyAt           time.Time
	RetryAt           time.Time
	Timeout           time.Duration
	Status            RunStatus
	Attempt           int
	ConcurrencyPolicy ConcurrencyPolicy
	MaxAttempts       int
	RetryPolicy       RetryPolicy
	LeaseOwner        string
	LeaseUntil        time.Time
	FencingToken      int64
	StartedAt         *time.Time
	FinishedAt        *time.Time
	ErrorClass        ErrorClass
	ErrorCode         string
	ErrorMessage      string
	ResultRef         string
	Output            []byte
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// RetryPolicy controls retry delay calculation.
type RetryPolicy struct {
	MaxAttempts int
	Initial     time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	Jitter      float64
}

// ClaimRequest describes a batch claim.
type ClaimRequest struct {
	OwnerID       string
	Limit         int
	LeaseDuration time.Duration
}

// ClaimedRun includes the ownership data required by an executor.
type ClaimedRun struct {
	Run          *Run
	OwnerID      string
	LeaseUntil   time.Time
	FencingToken int64
}

// ListDueRequest describes a due-definition query.
type ListDueRequest struct {
	Now   time.Time
	Limit int
}

// CreateRunRequest asks the store to atomically create the next run.
type CreateRunRequest struct {
	JobID     string
	PlannerID string
	Now       time.Time
}

// StartRunRequest transitions a leased run to running.
type StartRunRequest struct {
	RunID        string
	OwnerID      string
	FencingToken int64
}

// RenewLeaseRequest extends a run lease.
type RenewLeaseRequest struct {
	RunID         string
	OwnerID       string
	FencingToken  int64
	LeaseDuration time.Duration
}

// CompleteRunRequest commits a successful result.
type CompleteRunRequest struct {
	RunID        string
	OwnerID      string
	FencingToken int64
	ResultRef    string
	Output       []byte
}

// FailRunRequest records a failed attempt.
type FailRunRequest struct {
	RunID        string
	OwnerID      string
	FencingToken int64
	Class        ErrorClass
	Code         string
	Message      string
	RetryAt      time.Time
}

// CancelRunRequest cancels a pending or running run.
type CancelRunRequest struct {
	RunID string
}

// RecoverRequest describes an expired-run recovery batch.
type RecoverRequest struct {
	Now   time.Time
	Limit int
}

// CommitResult reports whether a conditional state write applied.
type CommitResult struct {
	Applied bool
	Run     *Run
}

// Capabilities describes the guarantees of a store.
type Capabilities struct {
	Transactions       bool
	MultiInstanceClaim bool
	ServerTime         bool
	AtomicSchedule     bool
	ConditionalWrite   bool
	BatchClaim         bool
	Durable            bool
	Outbox             bool
}

// StatsRequest selects store metrics.
type StatsRequest struct {
	Now time.Time
}

// Stats contains coarse queue metrics.
type Stats struct {
	Pending   int64
	RetryWait int64
	Running   int64
	Dead      int64
	QueueLag  time.Duration
}
