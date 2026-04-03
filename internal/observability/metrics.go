package observability

import (
	"github.com/prometheus/client_golang/prometheus"
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
)

// DefaultHTTPBuckets defines histogram buckets for HTTP request durations.
var DefaultHTTPBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// RemnawaveAPIBuckets defines histogram buckets for Remnawave API call durations.
var RemnawaveAPIBuckets = []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

// PluginHookBuckets defines histogram buckets for plugin hook execution durations.
var PluginHookBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 5}

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
}

// NewMetrics registers and returns the platform Prometheus metrics.
func NewMetrics() *Metrics {
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
	}
}
