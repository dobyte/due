package job

import (
	"context"
	"time"

	"github.com/dobyte/due/v2/errors"
)

// ArchiveRequest selects terminal runs that can be moved to history storage.
// Before is compared with the terminal update time and Limit bounds one batch.
type ArchiveRequest struct {
	Before time.Time
	Limit  int
}

// HistoryRequest selects archived runs.
type HistoryRequest struct {
	JobID  string
	Status RunStatus
	Before time.Time
	After  time.Time
	Limit  int
}

// PurgeHistoryRequest selects archived runs to delete permanently.
type PurgeHistoryRequest struct {
	Before time.Time
	Limit  int
}

// HistoryStore provides optional durable run history operations. Implementations
// must archive only terminal runs and must make each archive batch atomic when
// the backend supports transactions or server-side scripts.
type HistoryStore interface {
	ArchiveRuns(ctx context.Context, req ArchiveRequest) (int, error)
	GetHistory(ctx context.Context, runID string) (*Run, error)
	ListHistory(ctx context.Context, req HistoryRequest) ([]*Run, error)
	PurgeHistory(ctx context.Context, req PurgeHistoryRequest) (int, error)
}

// HistoryManager exposes optional history operations through Service.
type HistoryManager interface {
	ArchiveRuns(ctx context.Context, req ArchiveRequest) (int, error)
	GetHistory(ctx context.Context, runID string) (*Run, error)
	ListHistory(ctx context.Context, req HistoryRequest) ([]*Run, error)
	PurgeHistory(ctx context.Context, req PurgeHistoryRequest) (int, error)
}

var _ HistoryManager = (*Service)(nil)

func (s *Service) historyStore() (HistoryStore, error) {
	if s == nil || s.store == nil {
		return nil, errors.ErrInvalidArgument
	}
	history, ok := s.store.(HistoryStore)
	if !ok {
		return nil, ErrUnsupportedStore
	}
	return history, nil
}

// ArchiveRuns moves terminal runs to durable history storage.
func (s *Service) ArchiveRuns(ctx context.Context, req ArchiveRequest) (int, error) {
	history, err := s.historyStore()
	if err != nil {
		return 0, err
	}
	return history.ArchiveRuns(ctx, req)
}

// GetHistory returns one archived run.
func (s *Service) GetHistory(ctx context.Context, runID string) (*Run, error) {
	history, err := s.historyStore()
	if err != nil {
		return nil, err
	}
	return history.GetHistory(ctx, runID)
}

// ListHistory lists archived runs.
func (s *Service) ListHistory(ctx context.Context, req HistoryRequest) ([]*Run, error) {
	history, err := s.historyStore()
	if err != nil {
		return nil, err
	}
	return history.ListHistory(ctx, req)
}

// PurgeHistory permanently deletes archived runs.
func (s *Service) PurgeHistory(ctx context.Context, req PurgeHistoryRequest) (int, error) {
	history, err := s.historyStore()
	if err != nil {
		return 0, err
	}
	return history.PurgeHistory(ctx, req)
}

func historyLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	return limit
}

func terminalRun(status RunStatus) bool {
	switch status {
	case RunSucceeded, RunCanceled, RunDead:
		return true
	default:
		return false
	}
}
