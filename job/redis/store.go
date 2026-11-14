package redis

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/job"
	"github.com/dobyte/due/v2/utils/xuuid"
	"github.com/redis/go-redis/v9"
)

const (
	definitionDueSuffix = ":def:due"
	pendingSuffix       = ":run:pending"
	retrySuffix         = ":run:retry"
	leaseSuffix         = ":run:lease"
	terminalSuffix      = ":run:terminal"
	historySuffix       = ":run:history"
)

var (
	_ job.JobStore     = (*Store)(nil)
	_ job.HistoryStore = (*Store)(nil)

	createDefinitionScript = redis.NewScript(
		"if redis.call('EXISTS', KEYS[1]) == 1 then return 0 end " +
			"redis.call('HSET', KEYS[1], unpack(ARGV, 1, 34)) " +
			"if ARGV[35] == 'enabled' and ARGV[36] ~= '' then redis.call('ZADD', KEYS[2], ARGV[36], ARGV[2]) end " +
			"return 1",
	)
	updateDefinitionScript = redis.NewScript(
		"if redis.call('HGET', KEYS[1], 'version') ~= ARGV[1] then return 0 end " +
			"redis.call('HSET', KEYS[1], unpack(ARGV, 2, 35)) " +
			"redis.call('ZREM', KEYS[2], ARGV[36]) " +
			"if ARGV[37] == 'enabled' and ARGV[38] ~= '' then redis.call('ZADD', KEYS[2], ARGV[38], ARGV[36]) end " +
			"return 1",
	)
	createRunScript = redis.NewScript(
		"if redis.call('EXISTS', KEYS[1]) == 1 then return 0 end " +
			"redis.call('HSET', KEYS[1], unpack(ARGV, 1, 46)) " +
			"redis.call('ZADD', KEYS[2], ARGV[47], ARGV[2]) " +
			"return 1",
	)
	createRunIfDueScript = redis.NewScript(
		"if redis.call('HGET', KEYS[1], 'version') ~= ARGV[1] then return 0 end " +
			"if redis.call('GET', KEYS[2]) ~= false then return 0 end " +
			"if redis.call('HGET', KEYS[1], 'status') ~= 'enabled' then return 0 end " +
			"redis.call('HSET', KEYS[3], unpack(ARGV, 3, 48)) " +
			"redis.call('SET', KEYS[2], ARGV[2]) " +
			"redis.call('ZADD', KEYS[4], ARGV[49], ARGV[2]) " +
			"redis.call('HSET', KEYS[1], 'version', ARGV[50], 'next_run_at', ARGV[51], 'status', ARGV[52], 'updated_at', ARGV[53]) " +
			"redis.call('ZREM', KEYS[5], ARGV[54]) " +
			"if ARGV[52] == 'enabled' and ARGV[56] ~= '' then redis.call('ZADD', KEYS[5], ARGV[56], ARGV[54]) end " +
			"return 1",
	)
	promoteRetryScript = redis.NewScript(
		"local ids = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, ARGV[2]) " +
			"for _, id in ipairs(ids) do " +
			"local key = ARGV[3] .. id " +
			"if redis.call('HGET', key, 'status') == 'retry_wait' then " +
			"redis.call('HSET', key, 'status', 'pending', 'retry_at', '0', 'ready_at', ARGV[1], 'updated_at', ARGV[1]) " +
			"redis.call('ZREM', KEYS[1], id) redis.call('ZADD', KEYS[2], ARGV[1], id) " +
			"end end return #ids",
	)
	claimRunsScript = redis.NewScript(
		"local ids = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, ARGV[2]) " +
			"local result = {} " +
			"for _, id in ipairs(ids) do " +
			"local key = ARGV[6] .. id " +
			"local status = redis.call('HGET', key, 'status') " +
			"if status == 'pending' then " +
			"local job_id = redis.call('HGET', key, 'job_id') " +
			"local policy = redis.call('HGET', key, 'concurrency_policy') " +
			"local active = ARGV[7] .. job_id " +
			"if policy ~= 'forbid' or redis.call('EXISTS', active) == 0 then " +
			"local token = redis.call('HINCRBY', key, 'fencing_token', 1) " +
			"redis.call('HINCRBY', key, 'attempt', 1) " +
			"redis.call('HSET', key, 'status', 'leased', 'lease_owner', ARGV[3], 'lease_until', ARGV[4]) " +
			"redis.call('ZREM', KEYS[1], id) " +
			"redis.call('ZADD', KEYS[3], ARGV[4], id) " +
			"if policy == 'forbid' then redis.call('SET', active, id) end " +
			"table.insert(result, id) " +
			"table.insert(result, token) " +
			"end end end " +
			"return result",
	)
	archiveRunScript = redis.NewScript(
		"if redis.call('EXISTS', KEYS[1]) == 0 then redis.call('ZREM', KEYS[3], ARGV[2]) return 0 end " +
			"local values = redis.call('HGETALL', KEYS[1]) local updated = redis.call('HGET', KEYS[1], 'updated_at') " +
			"if #values > 0 then redis.call('HSET', KEYS[2], unpack(values)) end " +
			"redis.call('DEL', KEYS[1]) redis.call('ZREM', KEYS[3], ARGV[2]) redis.call('ZADD', KEYS[4], updated, ARGV[2]) return 1",
	)
	purgeHistoryScript = redis.NewScript(
		"if redis.call('EXISTS', KEYS[1]) == 1 then redis.call('DEL', KEYS[1]) end " +
			"redis.call('ZREM', KEYS[2], ARGV[1]) return 1",
	)
)

const runHashFields = 21

// Store is a Redis-backed JobStore.
type Store struct {
	opts    *options
	client  redis.UniversalClient
	builtin bool
	err     error
	mu      sync.Mutex
	closed  bool
}

// NewStore creates a Redis-backed JobStore.
func NewStore(opts ...Option) *Store {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Store{opts: o}
	s.client, s.builtin = clientFromOptions(context.Background(), o)
	return s
}

// Capabilities reports the guarantees provided by Store.
func (s *Store) Capabilities() job.Capabilities {
	return job.Capabilities{
		ServerTime:         false,
		AtomicSchedule:     true,
		ConditionalWrite:   true,
		MultiInstanceClaim: true,
		BatchClaim:         true,
		Durable:            true,
	}
}

// Close closes the built-in Redis client.
func (s *Store) Close(_ context.Context) error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	if s.builtin {
		return s.client.Close()
	}

	return nil
}

func (s *Store) check() error {
	if s == nil || s.client == nil {
		return errors.ErrInvalidArgument
	}

	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return job.ErrJobClosed
	}

	if s.err != nil {
		return s.err
	}

	return nil
}

func (s *Store) definitionKey(jobID string) string {
	return s.opts.prefix + ":def:" + jobID
}

func (s *Store) runKey(runID string) string {
	return s.opts.prefix + ":run:" + runID
}

func (s *Store) uniqueKey(jobID string, scheduledAt time.Time) string {
	return s.opts.prefix + ":uniq:" + jobID + ":" + strconv.FormatInt(scheduledAt.UnixMilli(), 10)
}

func (s *Store) activeKey(jobID string) string {
	return s.opts.prefix + ":active:" + jobID
}

func (s *Store) definitionsDueKey() string {
	return s.opts.prefix + definitionDueSuffix
}

func (s *Store) pendingKey() string {
	return s.opts.prefix + pendingSuffix
}

func (s *Store) retryKey() string {
	return s.opts.prefix + retrySuffix
}

func (s *Store) leaseKey() string {
	return s.opts.prefix + leaseSuffix
}

func (s *Store) terminalKey() string {
	return s.opts.prefix + terminalSuffix
}

func (s *Store) historyIndexKey() string {
	return s.opts.prefix + historySuffix
}

func (s *Store) historyKey(runID string) string {
	return s.historyIndexKey() + ":" + runID
}

func (s *Store) runPrefix() string {
	return s.opts.prefix + ":run:"
}

func (s *Store) activePrefix() string {
	return s.opts.prefix + ":active:"
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

	fields := definitionFields(definition)
	args := make([]any, 0, len(fields)+2)
	args = append(args, fields...)
	args = append(args, string(definition.Status), score(definition.NextRunAt))
	result, err := createDefinitionScript.Run(ctx, s.client, []string{
		s.definitionKey(definition.ID),
		s.definitionsDueKey(),
	}, args...).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return job.ErrDefinitionExists
	}

	return nil
}

func (s *Store) GetDefinition(ctx context.Context, jobID string) (*job.Definition, error) {
	if err := s.check(); err != nil {
		return nil, err
	}

	values, err := s.client.HGetAll(ctx, s.definitionKey(jobID)).Result()
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, job.ErrDefinitionNotFound
	}

	return decodeDefinition(values)
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
	fields := definitionFields(definition)
	args := []any{strconv.FormatInt(version, 10)}
	args = append(args, fields...)
	args = append(args, definition.ID, string(definition.Status), score(definition.NextRunAt))
	result, err := updateDefinitionScript.Run(ctx, s.client, []string{
		s.definitionKey(definition.ID),
		s.definitionsDueKey(),
	}, args...).Int()
	if err != nil {
		return err
	}
	if result == 0 {
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

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	values, err := s.client.ZRangeByScore(ctx, s.definitionsDueKey(), &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(req.Now.UnixMilli(), 10),
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, err
	}

	definitions := make([]*job.Definition, 0, len(values))
	for _, jobID := range values {
		definition, err := s.GetDefinition(ctx, jobID)
		if err != nil {
			if errors.Is(err, job.ErrDefinitionNotFound) {
				continue
			}
			return nil, err
		}
		if definition.Status == job.DefinitionEnabled && !definition.NextRunAt.After(req.Now) {
			definitions = append(definitions, definition)
		}
	}

	return definitions, nil
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

	fields := runFields(run)
	args := make([]any, 0, len(fields)+1)
	args = append(args, fields...)
	args = append(args, strconv.FormatInt(run.ReadyAt.UnixMilli(), 10))
	result, err := createRunScript.Run(ctx, s.client, []string{
		s.runKey(run.ID),
		s.pendingKey(),
	}, args...).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return job.ErrRunExists
	}

	return nil
}

func (s *Store) CreateRunIfDue(ctx context.Context, req job.CreateRunRequest) (*job.Run, bool, error) {
	definition, err := s.GetDefinition(ctx, req.JobID)
	if err != nil {
		return nil, false, err
	}
	if definition.Status != job.DefinitionEnabled || definition.NextRunAt.After(req.Now) {
		return nil, false, nil
	}

	scheduledAt := definition.NextRunAt
	nextRunAt, nextStatus, err := job.NextSchedule(definition, scheduledAt)
	if err != nil {
		return nil, false, err
	}

	run := &job.Run{
		ID:                newRunID(),
		JobID:             definition.ID,
		TaskType:          definition.Type,
		Payload:           append([]byte(nil), definition.Payload...),
		ScheduledAt:       scheduledAt,
		ReadyAt:           scheduledAt,
		Status:            job.RunPending,
		Timeout:           definition.Timeout,
		ConcurrencyPolicy: definition.ConcurrencyPolicy,
		MaxAttempts:       definition.MaxAttempts,
		RetryPolicy:       definition.RetryPolicy,
		CreatedAt:         req.Now,
		UpdatedAt:         req.Now,
	}
	next := *definition
	next.Version = definition.Version + 1
	next.NextRunAt = nextRunAt
	next.Status = nextStatus
	next.UpdatedAt = req.Now

	runFieldsList := runFields(run)
	args := []any{strconv.FormatInt(definition.Version, 10), run.ID}
	args = append(args, runFieldsList...)
	args = append(args,
		strconv.FormatInt(run.ReadyAt.UnixMilli(), 10),
		strconv.FormatInt(next.Version, 10),
		strconv.FormatInt(next.NextRunAt.UnixMilli(), 10),
		string(next.Status),
		strconv.FormatInt(req.Now.UnixMilli(), 10),
		definition.ID,
		string(next.Status),
		score(next.NextRunAt),
	)

	result, err := createRunIfDueScript.Run(ctx, s.client, []string{
		s.definitionKey(definition.ID),
		s.uniqueKey(definition.ID, scheduledAt),
		s.runKey(run.ID),
		s.pendingKey(),
		s.definitionsDueKey(),
	}, args...).Int()
	if err != nil {
		return nil, false, err
	}
	if result == 0 {
		return nil, false, nil
	}

	return run, true, nil
}

func (s *Store) GetRun(ctx context.Context, runID string) (*job.Run, error) {
	if err := s.check(); err != nil {
		return nil, err
	}

	values, err := s.client.HGetAll(ctx, s.runKey(runID)).Result()
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, job.ErrRunNotFound
	}

	return decodeRun(values)
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

	now := time.Now()
	leaseUntil := now.Add(req.LeaseDuration)
	if _, err := s.RecoverExpiredRuns(ctx, job.RecoverRequest{Now: now, Limit: req.Limit}); err != nil {
		return nil, err
	}
	if _, err := promoteRetryScript.Run(ctx, s.client, []string{
		s.retryKey(),
		s.pendingKey(),
	}, strconv.FormatInt(now.UnixMilli(), 10), strconv.Itoa(req.Limit), s.runPrefix()).Int(); err != nil {
		return nil, err
	}
	raw, err := claimRunsScript.Run(ctx, s.client, []string{
		s.pendingKey(),
		s.runPrefix(),
		s.leaseKey(),
	}, strconv.FormatInt(now.UnixMilli(), 10), strconv.Itoa(req.Limit), req.OwnerID,
		strconv.FormatInt(leaseUntil.UnixMilli(), 10), strconv.FormatInt(now.UnixMilli(), 10),
		s.runPrefix(), s.activePrefix()).Result()
	if err != nil {
		return nil, err
	}
	values, ok := raw.([]interface{})
	if !ok {
		return nil, errors.ErrInvalidFormat
	}

	result := make([]*job.ClaimedRun, 0, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		runID, ok := values[i].(string)
		if !ok {
			continue
		}
		token, err := strconv.ParseInt(toString(values[i+1]), 10, 64)
		if err != nil {
			return nil, err
		}
		run, err := s.GetRun(ctx, runID)
		if err != nil {
			return nil, err
		}
		result = append(result, &job.ClaimedRun{
			Run:          run,
			OwnerID:      req.OwnerID,
			LeaseUntil:   leaseUntil,
			FencingToken: token,
		})
	}

	return result, nil
}

func (s *Store) StartRun(ctx context.Context, req job.StartRunRequest) error {
	if err := s.check(); err != nil {
		return err
	}
	now := strconv.FormatInt(time.Now().UnixMilli(), 10)
	result, err := s.client.Eval(ctx,
		"if redis.call('HGET', KEYS[1], 'status') ~= 'leased' then return 0 end "+
			"if redis.call('HGET', KEYS[1], 'lease_owner') ~= ARGV[1] then return 0 end "+
			"if redis.call('HGET', KEYS[1], 'fencing_token') ~= ARGV[2] then return 0 end "+
			"redis.call('HSET', KEYS[1], 'status', 'running', 'started_at', ARGV[3], 'updated_at', ARGV[3]) return 1",
		[]string{s.runKey(req.RunID)},
		req.OwnerID, strconv.FormatInt(req.FencingToken, 10), now,
	).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return job.ErrLeaseLost
	}

	return nil
}

func (s *Store) RenewLease(ctx context.Context, req job.RenewLeaseRequest) error {
	if err := s.check(); err != nil {
		return err
	}
	leaseUntil := time.Now().Add(req.LeaseDuration)
	result, err := s.client.Eval(ctx,
		"if redis.call('HGET', KEYS[1], 'lease_owner') ~= ARGV[1] then return 0 end "+
			"if redis.call('HGET', KEYS[1], 'fencing_token') ~= ARGV[2] then return 0 end "+
			"if redis.call('HGET', KEYS[1], 'status') ~= 'running' then return 0 end "+
			"redis.call('HSET', KEYS[1], 'lease_until', ARGV[3], 'updated_at', ARGV[4]) "+
			"redis.call('ZADD', KEYS[2], ARGV[3], ARGV[5]) return 1",
		[]string{s.runKey(req.RunID), s.leaseKey()},
		req.OwnerID, strconv.FormatInt(req.FencingToken, 10),
		strconv.FormatInt(leaseUntil.UnixMilli(), 10),
		strconv.FormatInt(time.Now().UnixMilli(), 10), req.RunID,
	).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return job.ErrLeaseLost
	}

	return nil
}

func (s *Store) CompleteRun(ctx context.Context, req job.CompleteRunRequest) (job.CommitResult, error) {
	if err := s.check(); err != nil {
		return job.CommitResult{}, err
	}
	output := base64.StdEncoding.EncodeToString(req.Output)
	result, err := s.client.Eval(ctx,
		"if redis.call('HGET', KEYS[1], 'status') ~= 'running' then return 0 end "+
			"if redis.call('HGET', KEYS[1], 'lease_owner') ~= ARGV[1] then return 0 end "+
			"if redis.call('HGET', KEYS[1], 'fencing_token') ~= ARGV[2] then return 0 end "+
			"redis.call('HSET', KEYS[1], 'status', 'succeeded', 'finished_at', ARGV[3], 'updated_at', ARGV[3], 'result_ref', ARGV[4], 'output', ARGV[5]) "+
			"redis.call('ZREM', KEYS[2], ARGV[6]) redis.call('ZADD', KEYS[4], ARGV[3], ARGV[6]) "+
			"local job_id = redis.call('HGET', KEYS[1], 'job_id') "+
			"redis.call('DEL', KEYS[3] .. job_id) return 1",
		[]string{s.runKey(req.RunID), s.leaseKey(), s.activePrefix(), s.terminalKey()},
		req.OwnerID, strconv.FormatInt(req.FencingToken, 10),
		strconv.FormatInt(time.Now().UnixMilli(), 10), req.ResultRef, output, req.RunID,
	).Int()
	if err != nil {
		return job.CommitResult{}, err
	}

	return job.CommitResult{Applied: result == 1}, nil
}

func (s *Store) FailRun(ctx context.Context, req job.FailRunRequest) (job.CommitResult, error) {
	if err := s.check(); err != nil {
		return job.CommitResult{}, err
	}
	retryAt := score(req.RetryAt)
	result, err := s.client.Eval(ctx,
		"if redis.call('HGET', KEYS[1], 'lease_owner') ~= ARGV[1] then return 0 end "+
			"if redis.call('HGET', KEYS[1], 'fencing_token') ~= ARGV[2] then return 0 end "+
			"local status = 'dead' "+
			"if ARGV[3] == 'retryable' and ARGV[4] ~= '' then status = 'retry_wait' end "+
			"redis.call('HSET', KEYS[1], 'status', status, 'error_class', ARGV[3], 'error_code', ARGV[5], 'error_message', ARGV[6], 'retry_at', ARGV[4], 'finished_at', ARGV[7], 'updated_at', ARGV[7]) "+
			"redis.call('ZREM', KEYS[2], ARGV[8]) "+
			"if status == 'retry_wait' then redis.call('ZADD', KEYS[3], ARGV[4], ARGV[8]) end "+
			"local job_id = redis.call('HGET', KEYS[1], 'job_id') redis.call('DEL', KEYS[4] .. job_id) "+
			"if status == 'dead' then redis.call('ZADD', KEYS[5], ARGV[7], ARGV[8]) end return 1",
		[]string{s.runKey(req.RunID), s.leaseKey(), s.retryKey(), s.activePrefix(), s.terminalKey()},
		req.OwnerID, strconv.FormatInt(req.FencingToken, 10), string(req.Class), retryAt,
		req.Code, req.Message, strconv.FormatInt(time.Now().UnixMilli(), 10), req.RunID,
	).Int()
	if err != nil {
		return job.CommitResult{}, err
	}

	return job.CommitResult{Applied: result == 1}, nil
}

func (s *Store) CancelRun(ctx context.Context, req job.CancelRunRequest) error {
	if err := s.check(); err != nil {
		return err
	}
	result, err := s.client.Eval(ctx,
		"local status = redis.call('HGET', KEYS[1], 'status') "+
			"if status == false then return -1 end "+
			"if status == 'succeeded' or status == 'dead' or status == 'canceled' then return 0 end "+
			"redis.call('HSET', KEYS[1], 'status', 'canceled', 'finished_at', ARGV[1], 'updated_at', ARGV[1]) "+
			"redis.call('ZREM', KEYS[2], ARGV[2]) redis.call('ZREM', KEYS[3], ARGV[2]) redis.call('ZREM', KEYS[4], ARGV[2]) "+
			"redis.call('ZADD', KEYS[6], ARGV[1], ARGV[2]) local job_id = redis.call('HGET', KEYS[1], 'job_id') redis.call('DEL', KEYS[5] .. job_id) return 1",
		[]string{s.runKey(req.RunID), s.pendingKey(), s.retryKey(), s.leaseKey(), s.activePrefix(), s.terminalKey()},
		strconv.FormatInt(time.Now().UnixMilli(), 10), req.RunID,
	).Int()
	if err != nil {
		return err
	}
	if result < 0 {
		return job.ErrRunNotFound
	}

	return nil
}

func (s *Store) RecoverExpiredRuns(ctx context.Context, req job.RecoverRequest) (int, error) {
	if err := s.check(); err != nil {
		return 0, err
	}
	if req.Limit <= 0 {
		req.Limit = 100
	}
	ids, err := s.client.ZRangeByScore(ctx, s.leaseKey(), &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(req.Now.UnixMilli(), 10),
		Offset: 0,
		Count:  int64(req.Limit),
	}).Result()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, runID := range ids {
		result, err := s.client.Eval(ctx,
			"local status = redis.call('HGET', KEYS[1], 'status') "+
				"if status ~= 'leased' and status ~= 'running' then return 0 end "+
				"if tonumber(redis.call('HGET', KEYS[1], 'lease_until')) > tonumber(ARGV[1]) then return 0 end "+
				"redis.call('HSET', KEYS[1], 'status', 'pending', 'lease_owner', '', 'lease_until', '0', 'updated_at', ARGV[1]) "+
				"redis.call('ZREM', KEYS[2], ARGV[2]) redis.call('ZADD', KEYS[3], ARGV[1], ARGV[2]) "+
				"local job_id = redis.call('HGET', KEYS[1], 'job_id') redis.call('DEL', KEYS[4] .. job_id) return 1",
			[]string{s.runKey(runID), s.leaseKey(), s.pendingKey(), s.activePrefix()},
			strconv.FormatInt(req.Now.UnixMilli(), 10), runID,
		).Int()
		if err != nil {
			return count, err
		}
		count += result
	}

	return count, nil
}

func (s *Store) GetStats(ctx context.Context, req job.StatsRequest) (*job.Stats, error) {
	if err := s.check(); err != nil {
		return nil, err
	}

	pending, err := s.client.ZCard(ctx, s.pendingKey()).Result()
	if err != nil {
		return nil, err
	}
	retryWait, err := s.client.ZCard(ctx, s.retryKey()).Result()
	if err != nil {
		return nil, err
	}
	running, err := s.client.ZCard(ctx, s.leaseKey()).Result()
	if err != nil {
		return nil, err
	}

	return &job.Stats{
		Pending:   pending,
		RetryWait: retryWait,
		Running:   running,
	}, nil
}

func (s *Store) ArchiveRuns(ctx context.Context, req job.ArchiveRequest) (int, error) {
	if err := s.check(); err != nil {
		return 0, err
	}
	if req.Before.IsZero() {
		req.Before = time.Now()
	}
	limit := redisHistoryLimit(req.Limit)
	ids, err := s.client.ZRangeByScore(ctx, s.terminalKey(), &redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(req.Before.UnixMilli(), 10), Offset: 0, Count: int64(limit)}).Result()
	if err != nil {
		return 0, err
	}
	archived := 0
	for _, runID := range ids {
		result, err := archiveRunScript.Run(ctx, s.client, []string{s.runKey(runID), s.historyKey(runID), s.terminalKey(), s.historyIndexKey()}, strconv.FormatInt(req.Before.UnixMilli(), 10), runID).Int()
		if err != nil {
			return archived, err
		}
		archived += result
	}
	return archived, nil
}

func (s *Store) GetHistory(ctx context.Context, runID string) (*job.Run, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	values, err := s.client.HGetAll(ctx, s.historyKey(runID)).Result()
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, job.ErrRunNotFound
	}
	return decodeRun(values)
}

func (s *Store) ListHistory(ctx context.Context, req job.HistoryRequest) ([]*job.Run, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	limit := redisHistoryLimit(req.Limit)
	minScore, maxScore := "-inf", "+inf"
	if !req.After.IsZero() {
		minScore = strconv.FormatInt(req.After.UnixMilli(), 10)
	}
	if !req.Before.IsZero() {
		maxScore = strconv.FormatInt(req.Before.UnixMilli(), 10)
	}
	runs := make([]*job.Run, 0, limit)
	for offset := int64(0); len(runs) < limit; offset += int64(limit) {
		ids, err := s.client.ZRevRangeByScore(ctx, s.historyIndexKey(), &redis.ZRangeBy{Max: maxScore, Min: minScore, Offset: offset, Count: int64(limit)}).Result()
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			break
		}
		for _, runID := range ids {
			run, err := s.GetHistory(ctx, runID)
			if errors.Is(err, job.ErrRunNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if req.JobID != "" && run.JobID != req.JobID || req.Status != "" && run.Status != req.Status {
				continue
			}
			runs = append(runs, run)
			if len(runs) >= limit {
				break
			}
		}
		if len(ids) < limit {
			break
		}
	}
	return runs, nil
}

func (s *Store) PurgeHistory(ctx context.Context, req job.PurgeHistoryRequest) (int, error) {
	if err := s.check(); err != nil {
		return 0, err
	}
	if req.Before.IsZero() {
		req.Before = time.Now()
	}
	limit := redisHistoryLimit(req.Limit)
	ids, err := s.client.ZRangeByScore(ctx, s.historyIndexKey(), &redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(req.Before.UnixMilli(), 10), Offset: 0, Count: int64(limit)}).Result()
	if err != nil {
		return 0, err
	}
	purged := 0
	for _, runID := range ids {
		result, err := purgeHistoryScript.Run(ctx, s.client, []string{s.historyKey(runID), s.historyIndexKey()}, runID).Int()
		if err != nil {
			return purged, err
		}
		purged += result
	}
	return purged, nil
}

func redisHistoryLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	return limit
}

func definitionFields(definition *job.Definition) []any {
	return []any{
		"id", definition.ID,
		"name", definition.Name,
		"type", definition.Type,
		"payload", base64.StdEncoding.EncodeToString(definition.Payload),
		"schedule_type", string(definition.ScheduleType),
		"schedule_expr", definition.ScheduleExpr,
		"timezone", definition.Timezone,
		"next_run_at", strconv.FormatInt(definition.NextRunAt.UnixMilli(), 10),
		"status", string(definition.Status),
		"misfire_policy", string(definition.MisfirePolicy),
		"concurrency_policy", string(definition.ConcurrencyPolicy),
		"max_attempts", strconv.Itoa(definition.MaxAttempts),
		"retry_policy", encodeRetryPolicy(definition.RetryPolicy),
		"timeout", strconv.FormatInt(definition.Timeout.Milliseconds(), 10),
		"version", strconv.FormatInt(definition.Version, 10),
		"created_at", strconv.FormatInt(definition.CreatedAt.UnixMilli(), 10),
		"updated_at", strconv.FormatInt(definition.UpdatedAt.UnixMilli(), 10),
	}
}

func runFields(run *job.Run) []any {
	return []any{
		"id", run.ID,
		"job_id", run.JobID,
		"task_type", run.TaskType,
		"payload", base64.StdEncoding.EncodeToString(run.Payload),
		"scheduled_at", strconv.FormatInt(run.ScheduledAt.UnixMilli(), 10),
		"ready_at", strconv.FormatInt(run.ReadyAt.UnixMilli(), 10),
		"retry_at", strconv.FormatInt(run.RetryAt.UnixMilli(), 10),
		"timeout", strconv.FormatInt(run.Timeout.Milliseconds(), 10),
		"status", string(run.Status),
		"attempt", strconv.Itoa(run.Attempt),
		"concurrency_policy", string(run.ConcurrencyPolicy),
		"max_attempts", strconv.Itoa(run.MaxAttempts),
		"retry_policy", encodeRetryPolicy(run.RetryPolicy),
		"lease_owner", run.LeaseOwner,
		"lease_until", strconv.FormatInt(run.LeaseUntil.UnixMilli(), 10),
		"fencing_token", strconv.FormatInt(run.FencingToken, 10),
		"error_class", string(run.ErrorClass),
		"error_code", run.ErrorCode,
		"error_message", run.ErrorMessage,
		"result_ref", run.ResultRef,
		"output", base64.StdEncoding.EncodeToString(run.Output),
		"created_at", strconv.FormatInt(run.CreatedAt.UnixMilli(), 10),
		"updated_at", strconv.FormatInt(run.UpdatedAt.UnixMilli(), 10),
	}
}

func decodeDefinition(values map[string]string) (*job.Definition, error) {
	payload, err := base64.StdEncoding.DecodeString(values["payload"])
	if err != nil {
		return nil, err
	}

	return &job.Definition{
		ID:                values["id"],
		Name:              values["name"],
		Type:              values["type"],
		Payload:           payload,
		ScheduleType:      job.ScheduleType(values["schedule_type"]),
		ScheduleExpr:      values["schedule_expr"],
		Timezone:          values["timezone"],
		NextRunAt:         fromMillis(values["next_run_at"]),
		Status:            job.DefinitionStatus(values["status"]),
		MisfirePolicy:     job.MisfirePolicy(values["misfire_policy"]),
		ConcurrencyPolicy: job.ConcurrencyPolicy(values["concurrency_policy"]),
		MaxAttempts:       atoi(values["max_attempts"]),
		RetryPolicy:       decodeRetryPolicy(values["retry_policy"]),
		Timeout:           time.Duration(atoi64(values["timeout"])) * time.Millisecond,
		Version:           atoi64(values["version"]),
		CreatedAt:         fromMillis(values["created_at"]),
		UpdatedAt:         fromMillis(values["updated_at"]),
	}, nil
}

func decodeRun(values map[string]string) (*job.Run, error) {
	payload, err := base64.StdEncoding.DecodeString(values["payload"])
	if err != nil {
		return nil, err
	}
	output, err := base64.StdEncoding.DecodeString(values["output"])
	if err != nil {
		return nil, err
	}

	return &job.Run{
		ID:                values["id"],
		JobID:             values["job_id"],
		TaskType:          values["task_type"],
		Payload:           payload,
		ScheduledAt:       fromMillis(values["scheduled_at"]),
		ReadyAt:           fromMillis(values["ready_at"]),
		RetryAt:           fromMillis(values["retry_at"]),
		Timeout:           time.Duration(atoi64(values["timeout"])) * time.Millisecond,
		Status:            job.RunStatus(values["status"]),
		Attempt:           atoi(values["attempt"]),
		ConcurrencyPolicy: job.ConcurrencyPolicy(values["concurrency_policy"]),
		MaxAttempts:       atoi(values["max_attempts"]),
		RetryPolicy:       decodeRetryPolicy(values["retry_policy"]),
		LeaseOwner:        values["lease_owner"],
		LeaseUntil:        fromMillis(values["lease_until"]),
		FencingToken:      atoi64(values["fencing_token"]),
		ErrorClass:        job.ErrorClass(values["error_class"]),
		ErrorCode:         values["error_code"],
		ErrorMessage:      values["error_message"],
		ResultRef:         values["result_ref"],
		Output:            output,
		CreatedAt:         fromMillis(values["created_at"]),
		UpdatedAt:         fromMillis(values["updated_at"]),
	}, nil
}

func score(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return strconv.FormatInt(t.UnixMilli(), 10)
}

func fromMillis(value string) time.Time {
	milliseconds := atoi64(value)
	if milliseconds == 0 {
		return time.Time{}
	}

	return time.UnixMilli(milliseconds)
}

func atoi(value string) int {
	return int(atoi64(value))
}

func atoi64(value string) int64 {
	result, _ := strconv.ParseInt(value, 10, 64)
	return result
}

func encodeRetryPolicy(policy job.RetryPolicy) string {
	return strings.Join([]string{
		strconv.Itoa(policy.MaxAttempts),
		strconv.FormatInt(policy.Initial.Milliseconds(), 10),
		strconv.FormatInt(policy.MaxDelay.Milliseconds(), 10),
		strconv.FormatFloat(policy.Multiplier, 'g', -1, 64),
		strconv.FormatFloat(policy.Jitter, 'g', -1, 64),
	}, ",")
}

func decodeRetryPolicy(value string) job.RetryPolicy {
	parts := strings.Split(value, ",")
	if len(parts) != 5 {
		return job.RetryPolicy{}
	}

	multiplier, _ := strconv.ParseFloat(parts[3], 64)
	jitter, _ := strconv.ParseFloat(parts[4], 64)

	return job.RetryPolicy{
		MaxAttempts: atoi(parts[0]),
		Initial:     time.Duration(atoi64(parts[1])) * time.Millisecond,
		MaxDelay:    time.Duration(atoi64(parts[2])) * time.Millisecond,
		Multiplier:  multiplier,
		Jitter:      jitter,
	}
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	default:
		return ""
	}
}

func newRunID() string {
	return xuuid.UUID()
}
