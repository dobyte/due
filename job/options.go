package job

import (
	"time"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultWorkerID          = "job-worker"
	defaultPollInterval      = time.Second
	defaultRecoveryInterval  = 5 * time.Second
	defaultClaimBatchSize    = 100
	defaultPlannerBatchSize  = 100
	defaultLeaseDuration     = 30 * time.Second
	defaultRunnerConcurrency = 16
)

// Option configures a Service.
type Option func(*options)

// options contains Service configuration.
type options struct {
	store             JobStore
	notifier          Notifier
	metrics           Metrics
	logger            Logger
	workerID          string
	pollInterval      time.Duration
	recoveryInterval  time.Duration
	claimBatchSize    int
	plannerBatchSize  int
	leaseDuration     time.Duration
	runnerConcurrency int
}

// WithStore sets the persistent store.
func WithStore(store JobStore) Option {
	return func(o *options) {
		o.store = store
	}
}

// WithNotifier sets the optional wake-up notifier.
func WithNotifier(notifier Notifier) Option {
	return func(o *options) {
		o.notifier = notifier
	}
}

// WithMetrics sets the optional metrics sink.
func WithMetrics(metrics Metrics) Option {
	return func(o *options) {
		o.metrics = metrics
	}
}

// WithLogger sets the optional job logger.
func WithLogger(logger Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// WithWorkerID sets the worker identity used in leases.
func WithWorkerID(workerID string) Option {
	return func(o *options) {
		o.workerID = workerID
	}
}

// WithPollInterval sets the planner and runner polling interval.
func WithPollInterval(interval time.Duration) Option {
	return func(o *options) {
		if interval > 0 {
			o.pollInterval = interval
		}
	}
}

// WithRecoveryInterval sets the expired-run recovery interval.
func WithRecoveryInterval(interval time.Duration) Option {
	return func(o *options) {
		if interval > 0 {
			o.recoveryInterval = interval
		}
	}
}

// WithClaimBatchSize sets the maximum number of runs claimed per poll.
func WithClaimBatchSize(size int) Option {
	return func(o *options) {
		if size > 0 {
			o.claimBatchSize = size
		}
	}
}

// WithPlannerBatchSize sets the maximum number of definitions examined per poll.
func WithPlannerBatchSize(size int) Option {
	return func(o *options) {
		if size > 0 {
			o.plannerBatchSize = size
		}
	}
}

// WithLeaseDuration sets the run lease duration.
func WithLeaseDuration(duration time.Duration) Option {
	return func(o *options) {
		if duration > 0 {
			o.leaseDuration = duration
		}
	}
}

// WithRunnerConcurrency sets the local execution concurrency.
func WithRunnerConcurrency(concurrency int) Option {
	return func(o *options) {
		if concurrency > 0 {
			o.runnerConcurrency = concurrency
		}
	}
}

func defaultOptions() *options {
	return &options{
		workerID:          etc.Get("etc.job.workerID", defaultWorkerID).String(),
		pollInterval:      etc.Get("etc.job.pollInterval", defaultPollInterval).Duration(),
		recoveryInterval:  etc.Get("etc.job.recoveryInterval", defaultRecoveryInterval).Duration(),
		claimBatchSize:    etc.Get("etc.job.claimBatchSize", defaultClaimBatchSize).Int(),
		plannerBatchSize:  etc.Get("etc.job.plannerBatchSize", defaultPlannerBatchSize).Int(),
		leaseDuration:     etc.Get("etc.job.leaseDuration", defaultLeaseDuration).Duration(),
		runnerConcurrency: etc.Get("etc.job.runnerConcurrency", defaultRunnerConcurrency).Int(),
	}
}
