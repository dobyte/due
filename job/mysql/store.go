package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/job"
)

var _ job.JobStore = (*Store)(nil)

type Store struct {
	db         *sql.DB
	definition string
	run        string
	skipLocked bool
}

// NewStore creates a MySQL-backed JobStore using an injected database connection.
func NewStore(opts ...Option) *Store {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Store{
		db:         o.db,
		definition: o.tablePrefix + "_definition",
		run:        o.tablePrefix + "_run",
		skipLocked: o.skipLocked,
	}
}

// Capabilities reports the guarantees provided by Store.
func (s *Store) Capabilities() job.Capabilities {
	return job.Capabilities{
		Transactions:       true,
		MultiInstanceClaim: true,
		ServerTime:         true,
		AtomicSchedule:     true,
		ConditionalWrite:   true,
		BatchClaim:         true,
		Durable:            true,
		Outbox:             false,
	}
}

// Close does not close the injected database connection.
func (s *Store) Close(context.Context) error {
	return nil
}

func (s *Store) check() error {
	if s == nil || s.db == nil {
		return errors.ErrInvalidArgument
	}

	return nil
}

func (s *Store) CreateDefinition(ctx context.Context, definition *job.Definition) error {
	if err := s.check(); err != nil {
		return err
	}
	if err := job.ValidateDefinition(definition); err != nil {
		return err
	}

	now := time.Now()
	if definition.Status == "" {
		definition.Status = job.DefinitionEnabled
	}
	if definition.Version == 0 {
		definition.Version = 1
	}
	if definition.RetryPolicy.MaxAttempts == 0 {
		definition.RetryPolicy = job.DefaultRetryPolicy()
	}
	if definition.CreatedAt.IsZero() {
		definition.CreatedAt = now
	}
	if definition.UpdatedAt.IsZero() {
		definition.UpdatedAt = now
	}
	definition.Payload = bytesOrEmpty(definition.Payload)

	_, err := s.db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (job_id,name,task_type,payload,schedule_type,schedule_expr,timezone,next_run_at,status,misfire_policy,concurrency_policy,max_attempts,retry_policy,timeout_ms,version,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		s.definition,
	), definition.ID, definition.Name, definition.Type, definition.Payload,
		string(definition.ScheduleType), definition.ScheduleExpr, definition.Timezone,
		millis(definition.NextRunAt), string(definition.Status), string(definition.MisfirePolicy),
		string(definition.ConcurrencyPolicy), definition.MaxAttempts,
		encodeRetryPolicy(definition.RetryPolicy), definition.Timeout.Milliseconds(),
		definition.Version, millis(definition.CreatedAt), millis(definition.UpdatedAt),
	)
	return err
}

func (s *Store) GetDefinition(ctx context.Context, jobID string) (*job.Definition, error) {
	if err := s.check(); err != nil {
		return nil, err
	}

	row := s.db.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT job_id,name,task_type,payload,schedule_type,schedule_expr,timezone,next_run_at,status,misfire_policy,concurrency_policy,max_attempts,retry_policy,timeout_ms,version,created_at,updated_at FROM %s WHERE job_id=?",
		s.definition,
	), jobID)

	definition, err := scanDefinition(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, job.ErrDefinitionNotFound
	}
	return definition, err
}

func (s *Store) UpdateDefinition(ctx context.Context, definition *job.Definition, version int64) error {
	if err := s.check(); err != nil {
		return err
	}
	if err := job.ValidateDefinition(definition); err != nil {
		return err
	}

	definition.Version = version + 1
	definition.UpdatedAt = time.Now()
	definition.Payload = bytesOrEmpty(definition.Payload)
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(
		"UPDATE %s SET name=?,task_type=?,payload=?,schedule_type=?,schedule_expr=?,timezone=?,next_run_at=?,status=?,misfire_policy=?,concurrency_policy=?,max_attempts=?,retry_policy=?,timeout_ms=?,version=?,updated_at=? WHERE job_id=? AND version=?",
		s.definition,
	), definition.Name, definition.Type, definition.Payload, string(definition.ScheduleType),
		definition.ScheduleExpr, definition.Timezone, millis(definition.NextRunAt),
		string(definition.Status), string(definition.MisfirePolicy), string(definition.ConcurrencyPolicy),
		definition.MaxAttempts, encodeRetryPolicy(definition.RetryPolicy), definition.Timeout.Milliseconds(),
		definition.Version, millis(definition.UpdatedAt), definition.ID, version,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return job.ErrVersionConflict
	}

	return nil
}

func (s *Store) PauseDefinition(ctx context.Context, jobID string, version int64) error {
	definition, err := s.GetDefinition(ctx, jobID)
	if err != nil {
		return err
	}
	definition.Status = job.DefinitionPaused
	return s.UpdateDefinition(ctx, definition, version)
}

func (s *Store) ResumeDefinition(ctx context.Context, jobID string, version int64) error {
	definition, err := s.GetDefinition(ctx, jobID)
	if err != nil {
		return err
	}
	definition.Status = job.DefinitionEnabled
	return s.UpdateDefinition(ctx, definition, version)
}

func (s *Store) DeleteDefinition(ctx context.Context, jobID string, version int64) error {
	definition, err := s.GetDefinition(ctx, jobID)
	if err != nil {
		return err
	}
	definition.Status = job.DefinitionDeleted
	return s.UpdateDefinition(ctx, definition, version)
}

func (s *Store) ListDueDefinitions(ctx context.Context, req job.ListDueRequest) ([]*job.Definition, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if req.Now.IsZero() {
		req.Now = time.Now()
	}
	if req.Limit <= 0 {
		req.Limit = 100
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		"SELECT job_id,name,task_type,payload,schedule_type,schedule_expr,timezone,next_run_at,status,misfire_policy,concurrency_policy,max_attempts,retry_policy,timeout_ms,version,created_at,updated_at FROM %s WHERE status=? AND next_run_at<=? ORDER BY next_run_at LIMIT ?",
		s.definition,
	), string(job.DefinitionEnabled), millis(req.Now), req.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	definitions := make([]*job.Definition, 0, req.Limit)
	for rows.Next() {
		definition, err := scanDefinition(rows)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}

	return definitions, rows.Err()
}

func (s *Store) CreateRun(ctx context.Context, run *job.Run) error {
	if err := s.check(); err != nil {
		return err
	}
	if run == nil || run.ID == "" || run.JobID == "" || run.TaskType == "" {
		return job.ErrInvalidRun
	}
	if run.Status == "" {
		run.Status = job.RunPending
	}
	if run.RetryPolicy.MaxAttempts == 0 {
		run.RetryPolicy = job.DefaultRetryPolicy()
	}
	if run.ReadyAt.IsZero() {
		run.ReadyAt = time.Now()
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = time.Now()
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = run.CreatedAt
	}
	run.Payload = bytesOrEmpty(run.Payload)
	run.Output = bytesOrEmpty(run.Output)

	_, err := s.db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (run_id,job_id,task_type,payload,scheduled_at,ready_at,retry_at,timeout_ms,status,attempt,concurrency_policy,max_attempts,retry_policy,lease_owner,lease_until,fencing_token,started_at,finished_at,error_class,error_code,error_message,result_ref,output,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		s.run,
	), run.ID, run.JobID, run.TaskType, run.Payload, millis(run.ScheduledAt), millis(run.ReadyAt),
		millis(run.RetryAt), run.Timeout.Milliseconds(), string(run.Status), run.Attempt,
		string(run.ConcurrencyPolicy), run.MaxAttempts, encodeRetryPolicy(run.RetryPolicy),
		run.LeaseOwner, millis(run.LeaseUntil), run.FencingToken, millisPtr(run.StartedAt),
		millisPtr(run.FinishedAt), string(run.ErrorClass), run.ErrorCode, run.ErrorMessage,
		run.ResultRef, run.Output, millis(run.CreatedAt), millis(run.UpdatedAt),
	)
	return err
}

func (s *Store) CreateRunIfDue(ctx context.Context, req job.CreateRunRequest) (*job.Run, bool, error) {
	if err := s.check(); err != nil {
		return nil, false, err
	}
	if req.Now.IsZero() {
		req.Now = time.Now()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	definition, err := scanDefinition(tx.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT job_id,name,task_type,payload,schedule_type,schedule_expr,timezone,next_run_at,status,misfire_policy,concurrency_policy,max_attempts,retry_policy,timeout_ms,version,created_at,updated_at FROM %s WHERE job_id=? FOR UPDATE",
		s.definition,
	), req.JobID))
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, job.ErrDefinitionNotFound
		}
		return nil, false, err
	}
	if definition.Status != job.DefinitionEnabled || definition.NextRunAt.After(req.Now) {
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	}

	nextRunAt, nextStatus, err := job.NextSchedule(definition, definition.NextRunAt)
	if err != nil {
		_ = tx.Rollback()
		return nil, false, err
	}

	run := &job.Run{
		ID:                newRunID(),
		JobID:             definition.ID,
		TaskType:          definition.Type,
		Payload:           append([]byte(nil), definition.Payload...),
		ScheduledAt:       definition.NextRunAt,
		ReadyAt:           definition.NextRunAt,
		Timeout:           definition.Timeout,
		Status:            job.RunPending,
		ConcurrencyPolicy: definition.ConcurrencyPolicy,
		MaxAttempts:       definition.MaxAttempts,
		RetryPolicy:       definition.RetryPolicy,
		CreatedAt:         req.Now,
		UpdatedAt:         req.Now,
	}
	run.Payload = bytesOrEmpty(run.Payload)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (run_id,job_id,task_type,payload,scheduled_at,ready_at,retry_at,timeout_ms,status,attempt,concurrency_policy,max_attempts,retry_policy,lease_owner,lease_until,fencing_token,started_at,finished_at,error_class,error_code,error_message,result_ref,output,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		s.run,
	), run.ID, run.JobID, run.TaskType, run.Payload, millis(run.ScheduledAt), millis(run.ReadyAt),
		0, run.Timeout.Milliseconds(), string(run.Status), run.Attempt, string(run.ConcurrencyPolicy),
		run.MaxAttempts, encodeRetryPolicy(run.RetryPolicy), "", 0, 0, 0, 0, "", "", "", "", []byte{},
		millis(run.CreatedAt), millis(run.UpdatedAt),
	); err != nil {
		_ = tx.Rollback()
		return nil, false, err
	}

	definition.Version++
	definition.NextRunAt = nextRunAt
	definition.Status = nextStatus
	definition.UpdatedAt = req.Now
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(
		"UPDATE %s SET next_run_at=?,status=?,version=?,updated_at=? WHERE job_id=? AND version=?",
		s.definition,
	), millis(definition.NextRunAt), string(definition.Status), definition.Version,
		millis(definition.UpdatedAt), definition.ID, definition.Version-1); err != nil {
		_ = tx.Rollback()
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}

	return run, true, nil
}

func (s *Store) GetRun(ctx context.Context, runID string) (*job.Run, error) {
	if err := s.check(); err != nil {
		return nil, err
	}

	row := s.db.QueryRowContext(ctx, fmt.Sprintf(
		"SELECT run_id,job_id,task_type,payload,scheduled_at,ready_at,retry_at,timeout_ms,status,attempt,concurrency_policy,max_attempts,retry_policy,lease_owner,lease_until,fencing_token,started_at,finished_at,error_class,error_code,error_message,result_ref,output,created_at,updated_at FROM %s WHERE run_id=?",
		s.run,
	), runID)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, job.ErrRunNotFound
	}
	return run, err
}

func (s *Store) ClaimRuns(ctx context.Context, req job.ClaimRequest) ([]*job.ClaimedRun, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if req.OwnerID == "" {
		return nil, errors.ErrInvalidArgument
	}
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.LeaseDuration <= 0 {
		req.LeaseDuration = 30 * time.Second
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	lockClause := " FOR UPDATE"
	if s.skipLocked {
		lockClause += " SKIP LOCKED"
	}
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(
		"SELECT run_id,job_id,task_type,payload,scheduled_at,ready_at,retry_at,timeout_ms,status,attempt,concurrency_policy,max_attempts,retry_policy,lease_owner,lease_until,fencing_token,started_at,finished_at,error_class,error_code,error_message,result_ref,output,created_at,updated_at FROM %s WHERE (status='pending' AND ready_at<=?) OR (status='retry_wait' AND retry_at<=?) OR (status IN ('leased','running') AND lease_until>0 AND lease_until<=?) ORDER BY ready_at LIMIT ?%s",
		s.run, lockClause,
	), millis(time.Now()), millis(time.Now()), millis(time.Now()), req.Limit)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	candidates := make([]*job.Run, 0, req.Limit)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		candidates = append(candidates, run)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	runs := make([]*job.ClaimedRun, 0, req.Limit)
	for _, run := range candidates {
		if run.ConcurrencyPolicy == job.ForbidConcurrent {
			var active int
			if err := tx.QueryRowContext(ctx, fmt.Sprintf(
				"SELECT COUNT(*) FROM %s WHERE job_id=? AND run_id<>? AND status IN ('leased','running')",
				s.run,
			), run.JobID, run.ID).Scan(&active); err != nil {
				_ = tx.Rollback()
				return nil, err
			}
			if active > 0 {
				continue
			}
		}

		now := time.Now()
		leaseUntil := now.Add(req.LeaseDuration)
		token := run.FencingToken + 1
		attempt := run.Attempt + 1
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(
			"UPDATE %s SET status='leased',lease_owner=?,lease_until=?,fencing_token=?,attempt=?,updated_at=? WHERE run_id=?",
			s.run,
		), req.OwnerID, millis(leaseUntil), token, attempt, millis(now), run.ID); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		run.Status = job.RunLeased
		run.LeaseOwner = req.OwnerID
		run.LeaseUntil = leaseUntil
		run.FencingToken = token
		run.Attempt = attempt
		runs = append(runs, &job.ClaimedRun{
			Run:          run,
			OwnerID:      req.OwnerID,
			LeaseUntil:   leaseUntil,
			FencingToken: token,
		})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return runs, nil
}

func (s *Store) StartRun(ctx context.Context, req job.StartRunRequest) error {
	if err := s.check(); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(
		"UPDATE %s SET status='running',started_at=?,updated_at=? WHERE run_id=? AND status='leased' AND lease_owner=? AND fencing_token=?",
		s.run,
	), millis(time.Now()), millis(time.Now()), req.RunID, req.OwnerID, req.FencingToken)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return job.ErrLeaseLost
	}

	return nil
}

func (s *Store) RenewLease(ctx context.Context, req job.RenewLeaseRequest) error {
	if err := s.check(); err != nil {
		return err
	}
	now := time.Now()
	leaseUntil := now.Add(req.LeaseDuration)
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(
		"UPDATE %s SET lease_until=?,updated_at=? WHERE run_id=? AND status='running' AND lease_owner=? AND fencing_token=?",
		s.run,
	), millis(leaseUntil), millis(now), req.RunID, req.OwnerID, req.FencingToken)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return job.ErrLeaseLost
	}

	return nil
}

func (s *Store) CompleteRun(ctx context.Context, req job.CompleteRunRequest) (job.CommitResult, error) {
	if err := s.check(); err != nil {
		return job.CommitResult{}, err
	}
	now := millis(time.Now())
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(
		"UPDATE %s SET status='succeeded',finished_at=?,updated_at=?,result_ref=?,output=? WHERE run_id=? AND status='running' AND lease_owner=? AND fencing_token=?",
		s.run,
	), now, now, req.ResultRef, req.Output, req.RunID, req.OwnerID, req.FencingToken)
	if err != nil {
		return job.CommitResult{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return job.CommitResult{}, err
	}

	return job.CommitResult{Applied: affected == 1}, nil
}

func (s *Store) FailRun(ctx context.Context, req job.FailRunRequest) (job.CommitResult, error) {
	if err := s.check(); err != nil {
		return job.CommitResult{}, err
	}
	status := job.RunDead
	finishedAt := millis(time.Now())
	retryAt := millis(req.RetryAt)
	if req.Class == job.ErrorRetryable && !req.RetryAt.IsZero() {
		status = job.RunRetryWait
		finishedAt = 0
	}
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(
		"UPDATE %s SET status=?,retry_at=?,finished_at=?,updated_at=?,error_class=?,error_code=?,error_message=? WHERE run_id=? AND lease_owner=? AND fencing_token=? AND status IN ('leased','running')",
		s.run,
	), string(status), retryAt, finishedAt, millis(time.Now()), string(req.Class), req.Code, req.Message,
		req.RunID, req.OwnerID, req.FencingToken)
	if err != nil {
		return job.CommitResult{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return job.CommitResult{}, err
	}

	return job.CommitResult{Applied: affected == 1}, nil
}

func (s *Store) CancelRun(ctx context.Context, req job.CancelRunRequest) error {
	if err := s.check(); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(
		"UPDATE %s SET status='canceled',updated_at=? WHERE run_id=? AND status NOT IN ('succeeded','dead','canceled')",
		s.run,
	), millis(time.Now()), req.RunID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		if _, err := s.GetRun(ctx, req.RunID); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) RecoverExpiredRuns(ctx context.Context, req job.RecoverRequest) (int, error) {
	if err := s.check(); err != nil {
		return 0, err
	}
	if req.Now.IsZero() {
		req.Now = time.Now()
	}
	if req.Limit <= 0 {
		req.Limit = 100
	}

	result, err := s.db.ExecContext(ctx, fmt.Sprintf(
		"UPDATE %s SET status='pending',lease_owner='',lease_until=0,ready_at=?,updated_at=? WHERE status IN ('leased','running') AND lease_until>0 AND lease_until<=? ORDER BY lease_until LIMIT ?",
		s.run,
	), millis(req.Now), millis(req.Now), millis(req.Now), req.Limit)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(affected), nil
}

func (s *Store) GetStats(ctx context.Context, req job.StatsRequest) (*job.Stats, error) {
	if err := s.check(); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		"SELECT status,COUNT(*) FROM %s GROUP BY status",
		s.run,
	))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := &job.Stats{}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		switch job.RunStatus(status) {
		case job.RunPending:
			stats.Pending = count
		case job.RunRetryWait:
			stats.RetryWait = count
		case job.RunLeased, job.RunRunning:
			stats.Running += count
		case job.RunDead:
			stats.Dead = count
		}
	}
	return stats, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDefinition(row rowScanner) (*job.Definition, error) {
	var (
		definition                                        job.Definition
		payload                                           []byte
		scheduleType, status, misfire, concurrency        string
		retryPolicy                                       string
		nextRunAt, timeout, version, createdAt, updatedAt int64
	)
	err := row.Scan(
		&definition.ID, &definition.Name, &definition.Type, &payload,
		&scheduleType, &definition.ScheduleExpr, &definition.Timezone, &nextRunAt,
		&status, &misfire, &concurrency, &definition.MaxAttempts, &retryPolicy,
		&timeout, &version, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	definition.Payload = append([]byte(nil), payload...)
	definition.ScheduleType = job.ScheduleType(scheduleType)
	definition.NextRunAt = fromMillis(nextRunAt)
	definition.Status = job.DefinitionStatus(status)
	definition.MisfirePolicy = job.MisfirePolicy(misfire)
	definition.ConcurrencyPolicy = job.ConcurrencyPolicy(concurrency)
	definition.RetryPolicy = decodeRetryPolicy(retryPolicy)
	definition.Timeout = time.Duration(timeout) * time.Millisecond
	definition.Version = version
	definition.CreatedAt = fromMillis(createdAt)
	definition.UpdatedAt = fromMillis(updatedAt)
	return &definition, nil
}

func scanRun(row rowScanner) (*job.Run, error) {
	var (
		run                                                                                                           job.Run
		payload, output                                                                                               []byte
		status, concurrency, retryPolicy                                                                              string
		scheduledAt, readyAt, retryAt, timeout, leaseUntil, fencingToken, startedAt, finishedAt, createdAt, updatedAt int64
	)
	err := row.Scan(
		&run.ID, &run.JobID, &run.TaskType, &payload, &scheduledAt, &readyAt, &retryAt,
		&timeout, &status, &run.Attempt, &concurrency, &run.MaxAttempts, &retryPolicy,
		&run.LeaseOwner, &leaseUntil, &fencingToken, &startedAt, &finishedAt,
		&run.ErrorClass, &run.ErrorCode, &run.ErrorMessage, &run.ResultRef, &output,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	run.Payload = append([]byte(nil), payload...)
	run.Output = append([]byte(nil), output...)
	run.ScheduledAt = fromMillis(scheduledAt)
	run.ReadyAt = fromMillis(readyAt)
	run.RetryAt = fromMillis(retryAt)
	run.Timeout = time.Duration(timeout) * time.Millisecond
	run.Status = job.RunStatus(status)
	run.ConcurrencyPolicy = job.ConcurrencyPolicy(concurrency)
	run.RetryPolicy = decodeRetryPolicy(retryPolicy)
	run.LeaseUntil = fromMillis(leaseUntil)
	run.FencingToken = fencingToken
	run.StartedAt = timePtr(startedAt)
	run.FinishedAt = timePtr(finishedAt)
	run.CreatedAt = fromMillis(createdAt)
	run.UpdatedAt = fromMillis(updatedAt)
	return &run, nil
}

func encodeRetryPolicy(policy job.RetryPolicy) string {
	return fmt.Sprintf("%d,%d,%d,%g,%g", policy.MaxAttempts, policy.Initial.Milliseconds(), policy.MaxDelay.Milliseconds(), policy.Multiplier, policy.Jitter)
}

func decodeRetryPolicy(value string) job.RetryPolicy {
	parts := strings.Split(value, ",")
	if len(parts) != 5 {
		return job.RetryPolicy{}
	}
	var multiplier, jitter float64
	_, _ = fmt.Sscanf(parts[3], "%f", &multiplier)
	_, _ = fmt.Sscanf(parts[4], "%f", &jitter)
	return job.RetryPolicy{
		MaxAttempts: atoi(parts[0]),
		Initial:     time.Duration(atoi64(parts[1])) * time.Millisecond,
		MaxDelay:    time.Duration(atoi64(parts[2])) * time.Millisecond,
		Multiplier:  multiplier,
		Jitter:      jitter,
	}
}

func atoi(value string) int {
	return int(atoi64(value))
}

func atoi64(value string) int64 {
	result, _ := strconv.ParseInt(value, 10, 64)
	return result
}

func bytesOrEmpty(value []byte) []byte {
	if value == nil {
		return []byte{}
	}
	return value
}

func millis(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func millisPtr(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return millis(*value)
}

func fromMillis(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}

func timePtr(value int64) *time.Time {
	if value == 0 {
		return nil
	}
	result := fromMillis(value)
	return &result
}

func newRunID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
