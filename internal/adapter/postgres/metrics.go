package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsNamespace = "platform"
	metricsSubsystem = "postgres"
)

// MetricsCollector implements prometheus.Collector and reports pgxpool
// connection pool statistics on every Prometheus scrape. This avoids a
// background goroutine — metrics are fetched lazily when scraped.
type MetricsCollector struct {
	pool *pgxpool.Pool

	totalConns      *prometheus.Desc
	idleConns       *prometheus.Desc
	acquireCount    *prometheus.Desc
	acquireDuration *prometheus.Desc
	maxConns        *prometheus.Desc
}

// NewMetricsCollector returns a collector that exposes pgxpool connection stats.
// It must be registered with prometheus.Register or promauto equivalent.
func NewMetricsCollector(pool *pgxpool.Pool) *MetricsCollector {
	return &MetricsCollector{
		pool: pool,
		totalConns: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "pool_connections"),
			"Current number of connections in the pool.",
			nil, nil,
		),
		idleConns: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "pool_idle_connections"),
			"Current number of idle connections in the pool.",
			nil, nil,
		),
		acquireCount: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "pool_acquire_total"),
			"Cumulative count of connection acquires from the pool.",
			nil, nil,
		),
		acquireDuration: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "pool_acquire_duration_seconds_total"),
			"Cumulative time spent acquiring connections from the pool.",
			nil, nil,
		),
		maxConns: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "pool_max_connections"),
			"Maximum number of connections allowed in the pool.",
			nil, nil,
		),
	}
}

// Describe sends the metric descriptors to the channel.
func (c *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.totalConns
	ch <- c.idleConns
	ch <- c.acquireCount
	ch <- c.acquireDuration
	ch <- c.maxConns
}

// Collect fetches current pool stats from pgxpool and sends them as metrics.
func (c *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.pool.Stat()

	ch <- prometheus.MustNewConstMetric(c.totalConns, prometheus.GaugeValue, float64(stats.TotalConns()))
	ch <- prometheus.MustNewConstMetric(c.idleConns, prometheus.GaugeValue, float64(stats.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.acquireCount, prometheus.CounterValue, float64(stats.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(c.acquireDuration, prometheus.CounterValue, stats.AcquireDuration().Seconds())
	ch <- prometheus.MustNewConstMetric(c.maxConns, prometheus.GaugeValue, float64(stats.MaxConns()))
}
