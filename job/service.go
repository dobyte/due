package job

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	dueerrors "github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xuuid"
)

// Service manages definitions and local job execution.
type Service struct {
	opts     *options
	store    JobStore
	registry *Registry

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
	sem     chan struct{}
}

// JobService is the public name for Service.
type JobService = Service

// Registry maps task types to local handlers.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// HandlerRegistry is the public name for Registry.
type HandlerRegistry = Registry

// NewRegistry creates an empty Handler registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register installs a Handler for taskType.
func (r *Registry) Register(taskType string, handler Handler) error {
	if r == nil || taskType == "" || handler == nil {
		return dueerrors.ErrInvalidArgument
	}

	r.mu.Lock()
	r.handlers[taskType] = handler
	r.mu.Unlock()

	return nil
}

// Get returns the Handler for taskType.
func (r *Registry) Get(taskType string) (Handler, bool) {
	if r == nil {
		return nil, false
	}

	r.mu.RLock()
	handler, ok := r.handlers[taskType]
	r.mu.RUnlock()

	return handler, ok
}

// NewService creates a job Service.
func NewService(opts ...Option) (*Service, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.store == nil {
		return nil, dueerrors.ErrInvalidArgument
	}
	if !storeSupported(o.store.Capabilities()) {
		return nil, ErrUnsupportedStore
	}

	return &Service{
		opts:     o,
		store:    o.store,
		registry: NewRegistry(),
		sem:      make(chan struct{}, o.runnerConcurrency),
	}, nil
}

// Register installs a Handler in the Service registry.
func (s *Service) Register(taskType string, handler Handler) error {
	if s == nil {
		return dueerrors.ErrInvalidArgument
	}

	return s.registry.Register(taskType, handler)
}

// Start starts the Planner, Runner, and Recovery loops.
func (s *Service) Start(ctx context.Context) error {
	if s == nil || s.store == nil {
		return dueerrors.ErrInvalidArgument
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !s.running.CompareAndSwap(false, true) {
		return ErrJobStarted
	}

	s.mu.Lock()
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	s.wg.Go(s.planLoop)
	s.wg.Go(s.runLoop)
	s.wg.Go(s.recoveryLoop)

	return nil
}

// Close stops loops and waits for local executions to finish.
func (s *Service) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !s.running.CompareAndSwap(true, false) {
		return nil
	}

	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	wait := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(wait)
	}()

	select {
	case <-wait:
	case <-ctx.Done():
		return ctx.Err()
	}

	if s.opts.notifier != nil {
		if err := s.opts.notifier.Close(ctx); err != nil {
			return err
		}
	}

	return s.store.Close(ctx)
}

// Create creates a definition and wakes the Worker when a notifier is configured.
func (s *Service) Create(ctx context.Context, definition *Definition) error {
	if err := ValidateDefinition(definition); err != nil {
		return err
	}
	if definition.CreatedAt.IsZero() {
		definition.CreatedAt = time.Now()
	}
	if definition.UpdatedAt.IsZero() {
		definition.UpdatedAt = definition.CreatedAt
	}
	if definition.Version == 0 {
		definition.Version = 1
	}
	if definition.RetryPolicy.MaxAttempts == 0 {
		definition.RetryPolicy = DefaultRetryPolicy()
	}

	if err := s.store.CreateDefinition(ctx, cloneDefinition(definition)); err != nil {
		return err
	}
	if s.opts.notifier != nil {
		return s.opts.notifier.Notify(ctx, definition.ID)
	}

	return nil
}

// Trigger creates a one-off Run without changing the definition schedule.
func (s *Service) Trigger(ctx context.Context, jobID string) (*Run, error) {
	definition, err := s.store.GetDefinition(ctx, jobID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	run := &Run{
		ID:                xuuid.UUID(),
		JobID:             definition.ID,
		TaskType:          definition.Type,
		Payload:           append([]byte(nil), definition.Payload...),
		ScheduledAt:       now,
		ReadyAt:           now,
		Status:            RunPending,
		CreatedAt:         now,
		UpdatedAt:         now,
		ConcurrencyPolicy: definition.ConcurrencyPolicy,
		MaxAttempts:       definition.MaxAttempts,
		RetryPolicy:       definition.RetryPolicy,
		Timeout:           definition.Timeout,
	}
	if err := s.store.CreateRun(ctx, run); err != nil {
		return nil, err
	}

	return run, nil
}

func (s *Service) planLoop() {
	ticker := time.NewTicker(s.opts.pollInterval)
	defer ticker.Stop()

	for {
		if err := s.planOnce(s.currentContext()); err != nil && !dueerrors.Is(err, context.Canceled) {
			continue
		}

		select {
		case <-s.currentContext().Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) runLoop() {
	ticker := time.NewTicker(s.opts.pollInterval)
	defer ticker.Stop()

	for {
		if err := s.runOnce(s.currentContext()); err != nil && !dueerrors.Is(err, context.Canceled) {
			continue
		}

		select {
		case <-s.currentContext().Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) recoveryLoop() {
	ticker := time.NewTicker(s.opts.recoveryInterval)
	defer ticker.Stop()

	for {
		_, _ = s.store.RecoverExpiredRuns(s.currentContext(), RecoverRequest{
			Now:   time.Now(),
			Limit: s.opts.claimBatchSize,
		})

		select {
		case <-s.currentContext().Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) planOnce(ctx context.Context) error {
	definitions, err := s.store.ListDueDefinitions(ctx, ListDueRequest{
		Now:   time.Now(),
		Limit: s.opts.plannerBatchSize,
	})
	if err != nil {
		return err
	}

	for _, definition := range definitions {
		if _, _, err := s.store.CreateRunIfDue(ctx, CreateRunRequest{
			JobID:     definition.ID,
			PlannerID: s.opts.workerID,
			Now:       time.Now(),
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) runOnce(ctx context.Context) error {
	claimed, err := s.store.ClaimRuns(ctx, ClaimRequest{
		OwnerID:       s.opts.workerID,
		Limit:         s.opts.claimBatchSize,
		LeaseDuration: s.opts.leaseDuration,
	})
	if err != nil {
		return err
	}

	for _, run := range claimed {
		select {
		case s.sem <- struct{}{}:
			s.wg.Go(func() {
				defer func() { <-s.sem }()
				s.executeRun(run)
			})
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (s *Service) executeRun(claimed *ClaimedRun) {
	if claimed == nil || claimed.Run == nil {
		return
	}

	ctx, cancel := context.WithTimeout(s.currentContext(), runTimeout(claimed.Run, s.opts.leaseDuration))
	defer cancel()

	if err := s.store.StartRun(ctx, StartRunRequest{
		RunID:        claimed.Run.ID,
		OwnerID:      claimed.OwnerID,
		FencingToken: claimed.FencingToken,
	}); err != nil {
		return
	}

	leaseCtx, stopLease := context.WithCancel(ctx)
	leaseDone := make(chan struct{})
	leaseErr := make(chan error, 1)
	go func() {
		if err := s.renewLease(leaseCtx, claimed, cancel); err != nil {
			leaseErr <- err
		}
		close(leaseDone)
	}()

	handler, ok := s.registry.Get(claimed.Run.TaskType)
	if !ok {
		stopLease()
		<-leaseDone
		_, _ = s.store.FailRun(context.Background(), FailRunRequest{
			RunID:        claimed.Run.ID,
			OwnerID:      claimed.OwnerID,
			FencingToken: claimed.FencingToken,
			Class:        ErrorNonRetryable,
			Code:         "handler_not_found",
			Message:      ErrHandlerNotFound.Error(),
		})
		return
	}

	result, err := handler.Execute(ctx, &Request{
		RunID:        claimed.Run.ID,
		JobID:        claimed.Run.JobID,
		TaskType:     claimed.Run.TaskType,
		Payload:      append([]byte(nil), claimed.Run.Payload...),
		Attempt:      claimed.Run.Attempt,
		FencingToken: claimed.FencingToken,
		Deadline:     time.Now().Add(runTimeout(claimed.Run, s.opts.leaseDuration)),
	})

	stopLease()
	<-leaseDone
	select {
	case <-leaseErr:
		err = NewRetryableError(ErrLeaseLost)
	default:
	}

	if err == nil && result != nil {
		_, _ = s.store.CompleteRun(context.Background(), CompleteRunRequest{
			RunID:        claimed.Run.ID,
			OwnerID:      claimed.OwnerID,
			FencingToken: claimed.FencingToken,
			ResultRef:    result.ResultRef,
			Output:       append([]byte(nil), result.Output...),
		})
		return
	}

	if err == nil {
		err = dueerrors.ErrInvalidArgument
	}
	class := ClassifyError(err)
	retryAt := time.Time{}
	if class == ErrorRetryable && claimed.Run.MaxAttempts > 0 && claimed.Run.Attempt < claimed.Run.MaxAttempts {
		retryAt = nextRetryAt(claimed.Run.Attempt, claimed.Run.RetryPolicy, time.Now())
	}
	_, _ = s.store.FailRun(context.Background(), FailRunRequest{
		RunID:        claimed.Run.ID,
		OwnerID:      claimed.OwnerID,
		FencingToken: claimed.FencingToken,
		Class:        class,
		Code:         "handler_failed",
		Message:      err.Error(),
		RetryAt:      retryAt,
	})
}

func (s *Service) renewLease(ctx context.Context, claimed *ClaimedRun, cancel context.CancelFunc) error {
	interval := s.opts.leaseDuration / 2
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.store.RenewLease(ctx, RenewLeaseRequest{
				RunID:         claimed.Run.ID,
				OwnerID:       claimed.OwnerID,
				FencingToken:  claimed.FencingToken,
				LeaseDuration: s.opts.leaseDuration,
			}); err != nil {
				cancel()
				return err
			}
		}
	}
}

func (s *Service) currentContext() context.Context {
	s.mu.Lock()
	ctx := s.ctx
	s.mu.Unlock()
	if ctx == nil {
		return context.Background()
	}

	return ctx
}

func storeSupported(c Capabilities) bool {
	return c.Durable && c.AtomicSchedule && c.ConditionalWrite && c.MultiInstanceClaim
}

func cloneDefinition(definition *Definition) *Definition {
	copy := *definition
	copy.Payload = append([]byte(nil), definition.Payload...)
	return &copy
}
