package job

import "time"

// MetricLabel is a low-cardinality metric label.
type MetricLabel struct {
	Name  string
	Value string
}

// Metrics receives optional job counters and duration observations.
// Implementations should keep label cardinality bounded.
type Metrics interface {
	Add(name string, value float64, labels ...MetricLabel)
	Observe(name string, value float64, labels ...MetricLabel)
}

// Logger is the optional logging surface used by Service.
// Due's log.Logger satisfies this interface.
type Logger interface {
	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
}

// NopMetrics discards all metric events.
type NopMetrics struct{}

func (NopMetrics) Add(string, float64, ...MetricLabel)     {}
func (NopMetrics) Observe(string, float64, ...MetricLabel) {}

const (
	metricDefinitionsPlanned = "job_definitions_planned_total"
	metricRunsClaimed        = "job_runs_claimed_total"
	metricRunsSucceeded      = "job_runs_succeeded_total"
	metricRunsFailed         = "job_runs_failed_total"
	metricRunsRetried        = "job_runs_retried_total"
	metricRunsCanceled       = "job_runs_canceled_total"
	metricLeaseLost          = "job_lease_lost_total"
	metricRecovery           = "job_recovery_total"
	metricHandlerRejected    = "job_handler_rejected_total"
	metricHandlerDuration    = "job_handler_duration_seconds"
)

func observeHandlerDuration(metrics Metrics, started time.Time) {
	if metrics == nil {
		return
	}
	metrics.Observe(metricHandlerDuration, time.Since(started).Seconds())
}
