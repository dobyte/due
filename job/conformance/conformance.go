// Package conformance contains the shared JobStore semantic test suite.
package conformance

import (
	"context"
	stderrors "errors"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/job"
	"github.com/dobyte/due/v2/utils/xuuid"
)

// Factory creates an isolated store for one conformance test. The name should
// be used as a key prefix, table prefix, or collection prefix by adapters.
type Factory func(ctx context.Context, name string) (job.JobStore, error)

// Run executes the common JobStore semantic checks for one backend.
func Run(t *testing.T, factory Factory) {
	t.Helper()
	if factory == nil {
		t.Fatal("conformance factory is nil")
	}

	t.Run("capabilities", func(t *testing.T) {
		store := newStore(t, factory)
		capabilities := store.Capabilities()
		if !capabilities.Durable || !capabilities.AtomicSchedule || !capabilities.ConditionalWrite || !capabilities.MultiInstanceClaim {
			t.Fatalf("store capabilities are insufficient: %+v", capabilities)
		}
	})

	t.Run("create_run_if_due_is_single_winner", func(t *testing.T) {
		store := newStore(t, factory)
		definition := newDefinition("once")
		mustNoError(t, store.CreateDefinition(context.Background(), definition))

		var (
			waitGroup sync.WaitGroup
			results   = make(chan bool, 2)
			errors    = make(chan error, 2)
		)
		for range 2 {
			waitGroup.Go(func() {
				_, created, err := store.CreateRunIfDue(context.Background(), job.CreateRunRequest{
					JobID: definition.ID,
					Now:   time.Now(),
				})
				results <- created
				errors <- err
			})
		}
		waitGroup.Wait()
		close(results)
		close(errors)

		createdCount := 0
		for err := range errors {
			mustNoError(t, err)
		}
		for created := range results {
			if created {
				createdCount++
			}
		}
		if createdCount != 1 {
			t.Fatalf("created winners = %d, want 1", createdCount)
		}
	})

	t.Run("paused_definition_does_not_schedule", func(t *testing.T) {
		store := newStore(t, factory)
		definition := newDefinition("paused")
		mustNoError(t, store.CreateDefinition(context.Background(), definition))
		mustNoError(t, store.PauseDefinition(context.Background(), definition.ID, definition.Version))

		_, created, err := store.CreateRunIfDue(context.Background(), job.CreateRunRequest{JobID: definition.ID, Now: time.Now()})
		mustNoError(t, err)
		if created {
			t.Fatal("paused definition created a run")
		}
	})

	t.Run("expired_lease_can_be_reclaimed_with_new_fencing_token", func(t *testing.T) {
		store := newStore(t, factory)
		run := newRun("lease")
		mustNoError(t, store.CreateRun(context.Background(), run))

		first := claimOne(t, store, "worker-a", 100*time.Millisecond)
		mustNoError(t, store.StartRun(context.Background(), job.StartRunRequest{RunID: first.Run.ID, OwnerID: first.OwnerID, FencingToken: first.FencingToken}))
		time.Sleep(180 * time.Millisecond)

		second := claimOne(t, store, "worker-b", time.Second)
		if second.FencingToken <= first.FencingToken {
			t.Fatalf("new fencing token = %d, old = %d", second.FencingToken, first.FencingToken)
		}
		oldCommit, err := store.CompleteRun(context.Background(), job.CompleteRunRequest{
			RunID: first.Run.ID, OwnerID: first.OwnerID, FencingToken: first.FencingToken,
		})
		mustNoError(t, err)
		if oldCommit.Applied {
			t.Fatal("stale fencing token completed the run")
		}
	})

	t.Run("retry_wait_can_be_claimed_after_retry_at", func(t *testing.T) {
		store := newStore(t, factory)
		run := newRun("retry")
		mustNoError(t, store.CreateRun(context.Background(), run))
		claimed := claimOne(t, store, "worker-a", time.Second)
		retryAt := time.Now().Add(120 * time.Millisecond)
		commit, err := store.FailRun(context.Background(), job.FailRunRequest{
			RunID: claimed.Run.ID, OwnerID: claimed.OwnerID, FencingToken: claimed.FencingToken,
			Class: job.ErrorRetryable, Code: "temporary", Message: "temporary", RetryAt: retryAt,
		})
		mustNoError(t, err)
		if !commit.Applied {
			t.Fatal("retry failure was not applied")
		}
		time.Sleep(180 * time.Millisecond)
		retried := claimOne(t, store, "worker-b", time.Second)
		if retried.Run.Attempt != 2 {
			t.Fatalf("retry attempt = %d, want 2", retried.Run.Attempt)
		}
	})

	t.Run("cancel_is_idempotent_and_terminal", func(t *testing.T) {
		store := newStore(t, factory)
		run := newRun("cancel")
		mustNoError(t, store.CreateRun(context.Background(), run))
		mustNoError(t, store.CancelRun(context.Background(), job.CancelRunRequest{RunID: run.ID}))
		mustNoError(t, store.CancelRun(context.Background(), job.CancelRunRequest{RunID: run.ID}))
		stored, err := store.GetRun(context.Background(), run.ID)
		mustNoError(t, err)
		if stored.Status != job.RunCanceled {
			t.Fatalf("run status = %q, want canceled", stored.Status)
		}
	})

	t.Run("terminal_runs_can_be_archived_and_purged", func(t *testing.T) {
		store := newStore(t, factory)
		history, ok := store.(job.HistoryStore)
		if !ok {
			t.Skip("store does not implement HistoryStore")
		}
		run := newRun("history")
		mustNoError(t, store.CreateRun(context.Background(), run))
		claimed := claimOne(t, store, "history-worker", time.Second)
		mustNoError(t, store.StartRun(context.Background(), job.StartRunRequest{RunID: claimed.Run.ID, OwnerID: claimed.OwnerID, FencingToken: claimed.FencingToken}))
		commit, err := store.CompleteRun(context.Background(), job.CompleteRunRequest{RunID: claimed.Run.ID, OwnerID: claimed.OwnerID, FencingToken: claimed.FencingToken, Output: []byte("history")})
		mustNoError(t, err)
		if !commit.Applied {
			t.Fatal("terminal run completion was not applied")
		}

		archived, err := history.ArchiveRuns(context.Background(), job.ArchiveRequest{Before: time.Now().Add(time.Second), Limit: 10})
		mustNoError(t, err)
		if archived != 1 {
			t.Fatalf("archived runs = %d, want 1", archived)
		}
		if _, err := store.GetRun(context.Background(), run.ID); !jobErrorIs(err, job.ErrRunNotFound) {
			t.Fatalf("primary run error = %v, want run not found", err)
		}
		stored, err := history.GetHistory(context.Background(), run.ID)
		mustNoError(t, err)
		if stored.Status != job.RunSucceeded || string(stored.Output) != "history" {
			t.Fatalf("archived run = %+v", stored)
		}
		listed, err := history.ListHistory(context.Background(), job.HistoryRequest{JobID: run.JobID, Limit: 10})
		mustNoError(t, err)
		if len(listed) != 1 {
			t.Fatalf("history rows = %d, want 1", len(listed))
		}
		purged, err := history.PurgeHistory(context.Background(), job.PurgeHistoryRequest{Before: time.Now().Add(time.Second), Limit: 10})
		mustNoError(t, err)
		if purged != 1 {
			t.Fatalf("purged rows = %d, want 1", purged)
		}
		if _, err := history.GetHistory(context.Background(), run.ID); !jobErrorIs(err, job.ErrRunNotFound) {
			t.Fatalf("purged history error = %v, want run not found", err)
		}
	})
}

func jobErrorIs(err, target error) bool {
	return stderrors.Is(err, target)
}

func newStore(t *testing.T, factory Factory) job.JobStore {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := factory(ctx, "job_conformance_"+xuuid.UUID())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close(context.Background()) })
	return store
}

func newDefinition(suffix string) *job.Definition {
	now := time.Now().Add(-time.Second)
	return &job.Definition{
		ID:                "job-" + suffix + "-" + xuuid.UUID(),
		Name:              "conformance " + suffix,
		Type:              "conformance",
		ScheduleType:      job.ScheduleOnce,
		NextRunAt:         now,
		Status:            job.DefinitionEnabled,
		MisfirePolicy:     job.MisfireFireOnce,
		ConcurrencyPolicy: job.AllowConcurrent,
		MaxAttempts:       3,
		RetryPolicy:       job.DefaultRetryPolicy(),
	}
}

func newRun(suffix string) *job.Run {
	now := time.Now()
	return &job.Run{
		ID:                "run-" + suffix + "-" + xuuid.UUID(),
		JobID:             "job-" + suffix + "-" + xuuid.UUID(),
		TaskType:          "conformance",
		ScheduledAt:       now,
		ReadyAt:           now,
		Status:            job.RunPending,
		ConcurrencyPolicy: job.AllowConcurrent,
		MaxAttempts:       3,
		RetryPolicy:       job.DefaultRetryPolicy(),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func claimOne(t *testing.T, store job.JobStore, owner string, lease time.Duration) *job.ClaimedRun {
	t.Helper()
	claimed, err := store.ClaimRuns(context.Background(), job.ClaimRequest{OwnerID: owner, Limit: 1, LeaseDuration: lease})
	mustNoError(t, err)
	if len(claimed) != 1 {
		t.Fatalf("claimed runs = %d, want 1", len(claimed))
	}
	return claimed[0]
}

func mustNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
