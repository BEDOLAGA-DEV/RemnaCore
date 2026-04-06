package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metric name constants.
const (
	MetricHTTPRequestsTotal    = "platform_http_requests_total"
	MetricHTTPRequestDuration  = "platform_http_request_duration_seconds"
	MetricRemnawaveAPITotal    = "platform_remnawave_api_requests_total"
	MetricRemnawaveAPIDuration = "platform_remnawave_api_duration_seconds"
	MetricPluginHookDuration   = "platform_plugin_hook_duration_seconds"
	MetricPluginHookErrors     = "platform_plugin_hook_errors_total"
	MetricPluginHookTotal      = "platform_plugin_hook_invocations_total"
	MetricPluginMemory           = "platform_plugin_memory_bytes"
	MetricEventPublishFailures   = "platform_event_publish_failures_total"
	MetricRateLimiterFallback    = "platform_rate_limiter_fallback_total"

	MetricEventsProcessedTotal   = "platform_events_processed_total"
	MetricEventProcessingLatency = "platform_event_processing_duration_seconds"
	MetricEntityLockWaitLatency  = "platform_entity_lock_wait_duration_seconds"
	MetricDLQPublishedTotal      = "platform_dlq_published_total"
	MetricIdempotencyHitTotal    = "platform_idempotency_hit_total"

	MetricDLQDepth                 = "platform_dlq_depth"
	MetricEventProcessingLag       = "platform_event_processing_lag_seconds"

	MetricOutboxRelayBatchSize     = "platform_outbox_relay_batch_size"
	MetricOutboxRelayBatchLatency  = "platform_outbox_relay_batch_duration_seconds"
	MetricOutboxRelayPublishErrors = "platform_outbox_relay_publish_errors_total"
	MetricOutboxRelayEmptyPolls    = "platform_outbox_relay_empty_polls_total"
)

// Metric help string constants.
const (
	helpHTTPRequestsTotal    = "Total number of HTTP requests handled."
	helpHTTPRequestDuration  = "Duration of HTTP requests in seconds."
	helpRemnawaveAPITotal    = "Total number of Remnawave API requests."
	helpRemnawaveAPIDuration = "Duration of Remnawave API requests in seconds."
	helpPluginHookDuration   = "Duration of plugin hook executions in seconds."
	helpPluginHookErrors     = "Total number of plugin hook execution errors."
	helpPluginHookTotal      = "Total number of plugin hook invocations."
	helpPluginMemory           = "Current memory usage of a plugin in bytes."
	helpEventPublishFailures   = "Total number of failed domain event publish attempts."

	// HelpRateLimiterFallback is exported so adapter/valkey can register the
	// counter with a consistent help string.
	HelpRateLimiterFallback = "Total number of rate limiter requests that fell back to allow due to circuit breaker."

	helpEventsProcessedTotal   = "Total number of consumed events by type and processing status."
	helpEventProcessingLatency = "Duration of event processing in seconds."
	helpEntityLockWaitLatency  = "Duration spent waiting for per-entity lock in seconds."
	helpDLQPublishedTotal      = "Total number of messages published to the dead-letter queue."
	helpIdempotencyHitTotal    = "Total number of duplicate events detected by idempotency check."

	helpDLQDepth                 = "Current number of unprocessed messages in the dead-letter queue."
	helpEventProcessingLag       = "Time between event creation and consumer processing in seconds."

	helpOutboxRelayBatchSize     = "Number of events in each outbox relay batch."
	helpOutboxRelayBatchLatency  = "Duration of outbox relay batch processing in seconds."
	helpOutboxRelayPublishErrors = "Total number of NATS publish errors in the outbox relay."
	helpOutboxRelayEmptyPolls    = "Total number of outbox relay polls that returned zero events."
)

// Label name constants.
const (
	LabelMethod   = "method"
	LabelPath     = "path"
	LabelStatus   = "status"
	LabelEndpoint = "endpoint"
	LabelPlugin   = "plugin"
	LabelHook     = "hook"
	LabelAction    = "action"
	LabelEventType = "event_type"
	LabelWorkerID  = "worker_id"
)

// DefaultHTTPBuckets defines histogram buckets for HTTP request durations.
var DefaultHTTPBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// RemnawaveAPIBuckets defines histogram buckets for Remnawave API call durations.
var RemnawaveAPIBuckets = []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

// PluginHookBuckets defines histogram buckets for plugin hook execution durations.
var PluginHookBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 5}

// EventProcessingBuckets defines histogram buckets for event consumer processing
// and entity lock wait durations. Range covers sub-millisecond dedup skips through
// multi-second Remnawave provisioning calls.
var EventProcessingBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// EventProcessingLagBuckets defines histogram buckets for the delay between event
// creation and consumer processing. Range covers sub-second near-realtime through
// multi-minute backlogs.
var EventProcessingLagBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 300}

// OutboxRelayBatchBuckets defines histogram buckets for outbox relay batch sizes
// (event count per batch). Range covers empty through full batches.
var OutboxRelayBatchBuckets = []float64{1, 5, 10, 25, 50, 100}

// OutboxRelayLatencyBuckets defines histogram buckets for outbox relay batch
// processing durations. Range covers fast DB-only batches through slow NATS publishes.
var OutboxRelayLatencyBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}

// Consumer metric status label values.
const (
	StatusSuccess          = "success"
	StatusFailed           = "failed"
	StatusSkippedDuplicate = "skipped_duplicate"
	StatusDLQ              = "dlq"
)

// Metrics holds Prometheus metric collectors for the platform.
type Metrics struct {
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	RemnawaveAPITotal    *prometheus.CounterVec
	RemnawaveAPIDuration *prometheus.HistogramVec
	PluginHookDuration   *prometheus.HistogramVec
	PluginHookErrors     *prometheus.CounterVec
	PluginHookTotal      *prometheus.CounterVec
	PluginMemoryBytes    *prometheus.GaugeVec
	EventPublishFailures *prometheus.CounterVec

	// Outbox relay metrics
	OutboxRelayBatchSize     *prometheus.HistogramVec
	OutboxRelayBatchLatency  *prometheus.HistogramVec
	OutboxRelayPublishErrors prometheus.Counter
	OutboxRelayEmptyPolls    prometheus.Counter

	// DLQ metrics
	DLQDepth prometheus.Gauge

	// Consumer metrics
	EventsProcessedTotal   *prometheus.CounterVec
	EventProcessingLatency *prometheus.HistogramVec
	EventProcessingLag     *prometheus.HistogramVec
	EntityLockWaitLatency  *prometheus.HistogramVec
	DLQPublishedTotal      prometheus.Counter
	IdempotencyHitTotal    *prometheus.CounterVec
}

// registerRuntimeCollectors replaces the default Go and process collectors with
// enhanced versions that expose runtime/metrics-based GC, memory, and scheduler
// metrics. This is essential for observing Go 1.26 Green Tea GC behavior.
//
// Exposed metric families include:
//   - go_gc_duration_seconds (GC pause duration histogram)
//   - go_gc_gogc_percent (current GOGC setting)
//   - go_gc_gomemlimit_bytes (current GOMEMLIMIT)
//   - go_memstats_* (heap, stack, GC stats)
//   - go_sched_* (scheduler latency)
var runtimeCollectorsOnce sync.Once

func registerRuntimeCollectors() {
	runtimeCollectorsOnce.Do(func() {
		// Unregister the default collectors so we can replace them with the
		// extended versions that include runtime/metrics GC and scheduler data.
		prometheus.Unregister(collectors.NewGoCollector())
		prometheus.Unregister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

		prometheus.MustRegister(collectors.NewGoCollector(
			collectors.WithGoCollectorRuntimeMetrics(
				collectors.MetricsGC,
				collectors.MetricsMemory,
				collectors.MetricsScheduler,
			),
		))
		prometheus.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	})
}

// NewMetrics registers and returns the platform Prometheus metrics.
func NewMetrics() *Metrics {
	registerRuntimeCollectors()

	return &Metrics{
		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: MetricHTTPRequestsTotal,
			Help: helpHTTPRequestsTotal,
		}, []string{LabelMethod, LabelPath, LabelStatus}),

		HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    MetricHTTPRequestDuration,
			Help:    helpHTTPRequestDuration,
			Buckets: DefaultHTTPBuckets,
		}, []string{LabelMethod, LabelPath}),

		RemnawaveAPITotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: MetricRemnawaveAPITotal,
			Help: helpRemnawaveAPITotal,
		}, []string{LabelEndpoint, LabelStatus}),

		RemnawaveAPIDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    MetricRemnawaveAPIDuration,
			Help:    helpRemnawaveAPIDuration,
			Buckets: RemnawaveAPIBuckets,
		}, []string{LabelEndpoint}),

		PluginHookDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    MetricPluginHookDuration,
			Help:    helpPluginHookDuration,
			Buckets: PluginHookBuckets,
		}, []string{LabelPlugin, LabelHook}),

		PluginHookErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: MetricPluginHookErrors,
			Help: helpPluginHookErrors,
		}, []string{LabelPlugin, LabelHook}),

		PluginHookTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: MetricPluginHookTotal,
			Help: helpPluginHookTotal,
		}, []string{LabelPlugin, LabelHook, LabelAction}),

		PluginMemoryBytes: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: MetricPluginMemory,
			Help: helpPluginMemory,
		}, []string{LabelPlugin}),

		EventPublishFailures: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: MetricEventPublishFailures,
			Help: helpEventPublishFailures,
		}, []string{LabelEventType}),

		DLQDepth: promauto.NewGauge(prometheus.GaugeOpts{
			Name: MetricDLQDepth,
			Help: helpDLQDepth,
		}),

		OutboxRelayBatchSize: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    MetricOutboxRelayBatchSize,
			Help:    helpOutboxRelayBatchSize,
			Buckets: OutboxRelayBatchBuckets,
		}, []string{LabelWorkerID}),

		OutboxRelayBatchLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    MetricOutboxRelayBatchLatency,
			Help:    helpOutboxRelayBatchLatency,
			Buckets: OutboxRelayLatencyBuckets,
		}, []string{LabelWorkerID}),

		OutboxRelayPublishErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: MetricOutboxRelayPublishErrors,
			Help: helpOutboxRelayPublishErrors,
		}),

		OutboxRelayEmptyPolls: promauto.NewCounter(prometheus.CounterOpts{
			Name: MetricOutboxRelayEmptyPolls,
			Help: helpOutboxRelayEmptyPolls,
		}),

		EventsProcessedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: MetricEventsProcessedTotal,
			Help: helpEventsProcessedTotal,
		}, []string{LabelEventType, LabelStatus}),

		EventProcessingLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    MetricEventProcessingLatency,
			Help:    helpEventProcessingLatency,
			Buckets: EventProcessingBuckets,
		}, []string{LabelEventType}),

		EventProcessingLag: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    MetricEventProcessingLag,
			Help:    helpEventProcessingLag,
			Buckets: EventProcessingLagBuckets,
		}, []string{LabelEventType}),

		EntityLockWaitLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    MetricEntityLockWaitLatency,
			Help:    helpEntityLockWaitLatency,
			Buckets: EventProcessingBuckets,
		}, []string{LabelEventType}),

		DLQPublishedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: MetricDLQPublishedTotal,
			Help: helpDLQPublishedTotal,
		}),

		IdempotencyHitTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: MetricIdempotencyHitTotal,
			Help: helpIdempotencyHitTotal,
		}, []string{LabelEventType}),
	}
}
