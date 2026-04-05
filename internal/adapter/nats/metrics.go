package nats

import (
	"github.com/prometheus/client_golang/prometheus"

	nc "github.com/nats-io/nats.go"
)

const (
	metricsNamespace = "platform"
	metricsSubsystem = "nats"
)

// MetricsCollector implements prometheus.Collector and reports NATS connection
// statistics on every Prometheus scrape. Metrics are fetched lazily — no
// background goroutine needed.
type MetricsCollector struct {
	conn *nc.Conn

	inMsgs      *prometheus.Desc
	outMsgs     *prometheus.Desc
	inBytes     *prometheus.Desc
	outBytes    *prometheus.Desc
	reconnects  *prometheus.Desc
}

// NewMetricsCollector returns a collector that exposes NATS connection stats.
func NewMetricsCollector(conn *nc.Conn) *MetricsCollector {
	return &MetricsCollector{
		conn: conn,
		inMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "messages_received_total"),
			"Total number of messages received from NATS.",
			nil, nil,
		),
		outMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "messages_sent_total"),
			"Total number of messages sent to NATS.",
			nil, nil,
		),
		inBytes: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "bytes_received_total"),
			"Total bytes received from NATS.",
			nil, nil,
		),
		outBytes: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "bytes_sent_total"),
			"Total bytes sent to NATS.",
			nil, nil,
		),
		reconnects: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "reconnects_total"),
			"Total number of reconnections to the NATS server.",
			nil, nil,
		),
	}
}

// Describe sends the metric descriptors to the channel.
func (c *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.inMsgs
	ch <- c.outMsgs
	ch <- c.inBytes
	ch <- c.outBytes
	ch <- c.reconnects
}

// Collect fetches current NATS connection stats and sends them as metrics.
func (c *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.conn.Statistics

	ch <- prometheus.MustNewConstMetric(c.inMsgs, prometheus.CounterValue, float64(stats.InMsgs))
	ch <- prometheus.MustNewConstMetric(c.outMsgs, prometheus.CounterValue, float64(stats.OutMsgs))
	ch <- prometheus.MustNewConstMetric(c.inBytes, prometheus.CounterValue, float64(stats.InBytes))
	ch <- prometheus.MustNewConstMetric(c.outBytes, prometheus.CounterValue, float64(stats.OutBytes))
	ch <- prometheus.MustNewConstMetric(c.reconnects, prometheus.CounterValue, float64(stats.Reconnects))
}
