package valkey

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

const (
	metricsNamespace = "platform"
	metricsSubsystem = "valkey"
)

// MetricsCollector implements prometheus.Collector and reports go-redis
// connection pool statistics on every Prometheus scrape. This avoids a
// background goroutine — metrics are fetched lazily when scraped.
type MetricsCollector struct {
	client *redis.Client

	poolHits       *prometheus.Desc
	poolMisses     *prometheus.Desc
	poolTimeouts   *prometheus.Desc
	poolTotalConns *prometheus.Desc
	poolIdleConns  *prometheus.Desc
	poolStaleConns *prometheus.Desc
}

// NewMetricsCollector returns a collector that exposes go-redis pool stats.
// It must be registered with prometheus.Register or promauto equivalent.
func NewMetricsCollector(client *redis.Client) *MetricsCollector {
	return &MetricsCollector{
		client: client,
		poolHits: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "pool_hits_total"),
			"Number of times a free connection was found in the pool.",
			nil, nil,
		),
		poolMisses: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "pool_misses_total"),
			"Number of times a free connection was NOT found in the pool.",
			nil, nil,
		),
		poolTimeouts: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "pool_timeouts_total"),
			"Number of times a wait for a connection timed out.",
			nil, nil,
		),
		poolTotalConns: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "pool_connections"),
			"Current number of connections in the pool.",
			nil, nil,
		),
		poolIdleConns: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "pool_idle_connections"),
			"Current number of idle connections in the pool.",
			nil, nil,
		),
		poolStaleConns: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "pool_stale_connections_total"),
			"Number of stale connections removed from the pool.",
			nil, nil,
		),
	}
}

// Describe sends the metric descriptors to the channel.
func (c *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.poolHits
	ch <- c.poolMisses
	ch <- c.poolTimeouts
	ch <- c.poolTotalConns
	ch <- c.poolIdleConns
	ch <- c.poolStaleConns
}

// Collect fetches current pool stats from go-redis and sends them as metrics.
func (c *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.client.PoolStats()

	ch <- prometheus.MustNewConstMetric(c.poolHits, prometheus.CounterValue, float64(stats.Hits))
	ch <- prometheus.MustNewConstMetric(c.poolMisses, prometheus.CounterValue, float64(stats.Misses))
	ch <- prometheus.MustNewConstMetric(c.poolTimeouts, prometheus.CounterValue, float64(stats.Timeouts))
	ch <- prometheus.MustNewConstMetric(c.poolTotalConns, prometheus.GaugeValue, float64(stats.TotalConns))
	ch <- prometheus.MustNewConstMetric(c.poolIdleConns, prometheus.GaugeValue, float64(stats.IdleConns))
	ch <- prometheus.MustNewConstMetric(c.poolStaleConns, prometheus.CounterValue, float64(stats.StaleConns))
}
