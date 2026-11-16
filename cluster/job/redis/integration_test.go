package redis

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster/job"
	"github.com/dobyte/due/v2/utils/xuuid"
)

func TestWorkerCrashRecovery(t *testing.T) {
	addr := integrationRedisAddr(t)
	prefix := "due:job:integration:{crash-" + xuuid.UUID() + "}"
	storeA := NewStore(WithAddrs(addr), WithPrefix(prefix))
	storeB := NewStore(WithAddrs(addr), WithPrefix(prefix))
	defer storeA.Close(context.Background())
	defer storeB.Close(context.Background())

	now := time.Now()
	run := &job.Run{
		ID:                "run-" + xuuid.UUID(),
		JobID:             "job-" + xuuid.UUID(),
		TaskType:          "crash-recovery",
		ScheduledAt:       now,
		ReadyAt:           now,
		Status:            job.RunPending,
		ConcurrencyPolicy: job.AllowConcurrent,
		MaxAttempts:       3,
		RetryPolicy:       job.DefaultRetryPolicy(),
	}
	if err := storeA.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	first, err := claimOneIntegration(storeA, "worker-a", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	// worker-a is intentionally abandoned without completing or renewing the run.
	time.Sleep(180 * time.Millisecond)
	second, err := claimOneIntegration(storeB, "worker-b", time.Second)
	if err != nil {
		t.Fatalf("recovery claim: %v", err)
	}
	if second.OwnerID != "worker-b" {
		t.Fatalf("owner = %q, want worker-b", second.OwnerID)
	}
	if second.FencingToken <= first.FencingToken {
		t.Fatalf("fencing token = %d, old = %d", second.FencingToken, first.FencingToken)
	}
}

func TestLongRunningHandlerRenewsLease(t *testing.T) {
	addr := integrationRedisAddr(t)
	prefix := "due:job:integration:{long-" + xuuid.UUID() + "}"
	store := NewStore(WithAddrs(addr), WithPrefix(prefix))
	service, err := job.NewService(
		job.WithStore(store),
		job.WithWorkerID("long-worker"),
		job.WithPollInterval(20*time.Millisecond),
		job.WithRecoveryInterval(50*time.Millisecond),
		job.WithLeaseDuration(250*time.Millisecond),
		job.WithRunnerConcurrency(1),
	)
	if err != nil {
		store.Close(context.Background())
		t.Fatalf("create service: %v", err)
	}

	handler := &blockingHandler{
		duration: 700 * time.Millisecond,
		started:  make(chan struct{}),
		finished: make(chan struct{}),
	}
	if err := service.Register("long-task", handler); err != nil {
		service.Close(context.Background())
		t.Fatalf("register handler: %v", err)
	}
	if err := service.Start(context.Background()); err != nil {
		service.Close(context.Background())
		t.Fatalf("start service: %v", err)
	}
	defer service.Close(context.Background())

	definition := &job.Definition{
		ID:                "job-" + xuuid.UUID(),
		Type:              "long-task",
		ScheduleType:      job.ScheduleOnce,
		NextRunAt:         time.Now().Add(-time.Second),
		Status:            job.DefinitionEnabled,
		MisfirePolicy:     job.MisfireFireOnce,
		ConcurrencyPolicy: job.AllowConcurrent,
		MaxAttempts:       1,
		RetryPolicy:       job.DefaultRetryPolicy(),
		Timeout:           2 * time.Second,
	}
	if err := service.Create(context.Background(), definition); err != nil {
		t.Fatalf("create definition: %v", err)
	}

	select {
	case <-handler.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for long handler")
	}
	select {
	case <-handler.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for long handler completion")
	}

	var run *job.Run
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if value := handler.runID.Load(); value != nil {
			run, err = store.GetRun(context.Background(), value.(string))
			if err == nil && run.Status == job.RunSucceeded {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if run == nil || run.Status != job.RunSucceeded {
		t.Fatalf("run did not succeed: %+v, err=%v", run, err)
	}
	if calls := handler.calls.Load(); calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}
	if run.Attempt != 1 || run.FencingToken != 1 {
		t.Fatalf("run attempt/token = %d/%d, want 1/1", run.Attempt, run.FencingToken)
	}
}

func TestSentinelStore(t *testing.T) {
	addresses := os.Getenv("JOB_REDIS_SENTINEL_ADDRS")
	masterName := os.Getenv("JOB_REDIS_SENTINEL_MASTER")
	if addresses == "" || masterName == "" {
		t.Skip("set JOB_REDIS_SENTINEL_ADDRS and JOB_REDIS_SENTINEL_MASTER to run Sentinel integration tests")
	}
	store := NewStore(
		WithAddrs(strings.Split(addresses, ",")...),
		WithSentinelMaster(masterName),
		WithPrefix("due:job:integration:{sentinel-"+xuuid.UUID()+"}"),
	)
	defer store.Close(context.Background())
	definition := &job.Definition{
		ID:                "job-" + xuuid.UUID(),
		Type:              "sentinel",
		ScheduleType:      job.ScheduleOnce,
		NextRunAt:         time.Now().Add(time.Hour),
		Status:            job.DefinitionEnabled,
		MisfirePolicy:     job.MisfireFireOnce,
		ConcurrencyPolicy: job.AllowConcurrent,
		MaxAttempts:       1,
	}
	if err := store.CreateDefinition(context.Background(), definition); err != nil {
		t.Fatalf("create definition through Sentinel: %v", err)
	}
	stored, err := store.GetDefinition(context.Background(), definition.ID)
	if err != nil {
		t.Fatalf("get definition through Sentinel: %v", err)
	}
	if stored.ID != definition.ID {
		t.Fatalf("stored job ID = %q, want %q", stored.ID, definition.ID)
	}
}

func integrationRedisAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("JOB_REDIS_ADDR")
	if addr == "" {
		t.Skip("set JOB_REDIS_ADDR to run Redis integration tests")
	}
	return addr
}

func claimOneIntegration(store job.JobStore, owner string, lease time.Duration) (*job.ClaimedRun, error) {
	claimed, err := store.ClaimRuns(context.Background(), job.ClaimRequest{OwnerID: owner, Limit: 1, LeaseDuration: lease})
	if err != nil {
		return nil, err
	}
	if len(claimed) != 1 {
		return nil, job.ErrRunNotFound
	}
	return claimed[0], nil
}

type blockingHandler struct {
	duration   time.Duration
	started    chan struct{}
	finished   chan struct{}
	calls      atomic.Int32
	runID      atomic.Value
	startOnce  sync.Once
	finishOnce sync.Once
}

func (h *blockingHandler) Execute(ctx context.Context, request *job.Request) (*job.Result, error) {
	h.calls.Add(1)
	h.runID.Store(request.RunID)
	h.startOnce.Do(func() { close(h.started) })
	defer h.finishOnce.Do(func() { close(h.finished) })
	timer := time.NewTimer(h.duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return &job.Result{Output: []byte("done")}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
