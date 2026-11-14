package job

import "context"

// JobStore persists definitions and runs and provides their atomic transitions.
type JobStore interface {
	Close(ctx context.Context) error
	Capabilities() Capabilities

	CreateDefinition(ctx context.Context, definition *Definition) error
	GetDefinition(ctx context.Context, jobID string) (*Definition, error)
	UpdateDefinition(ctx context.Context, definition *Definition, version int64) error
	PauseDefinition(ctx context.Context, jobID string, version int64) error
	ResumeDefinition(ctx context.Context, jobID string, version int64) error
	DeleteDefinition(ctx context.Context, jobID string, version int64) error

	ListDueDefinitions(ctx context.Context, req ListDueRequest) ([]*Definition, error)
	CreateRun(ctx context.Context, run *Run) error
	CreateRunIfDue(ctx context.Context, req CreateRunRequest) (*Run, bool, error)

	GetRun(ctx context.Context, runID string) (*Run, error)
	ClaimRuns(ctx context.Context, req ClaimRequest) ([]*ClaimedRun, error)
	StartRun(ctx context.Context, req StartRunRequest) error
	RenewLease(ctx context.Context, req RenewLeaseRequest) error
	CompleteRun(ctx context.Context, req CompleteRunRequest) (CommitResult, error)
	FailRun(ctx context.Context, req FailRunRequest) (CommitResult, error)
	CancelRun(ctx context.Context, req CancelRunRequest) error
	RecoverExpiredRuns(ctx context.Context, req RecoverRequest) (int, error)

	GetStats(ctx context.Context, req StatsRequest) (*Stats, error)
}

// Notifier wakes a running job service after a definition or run changes.
type Notifier interface {
	Notify(ctx context.Context, jobID string) error
	Close(ctx context.Context) error
}
