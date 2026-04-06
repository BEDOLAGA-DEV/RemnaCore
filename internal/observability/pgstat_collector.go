package observability

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// pgStatCollector constants.
const (
	// pgStatTopN limits the number of queries returned from pg_stat_statements.
	pgStatTopN = 10

	// pgStatQueryMaxLen truncates query text for label safety, preventing
	// high-cardinality explosion in Prometheus.
	pgStatQueryMaxLen = 80

	// pgStatQueryTimeout is the maximum duration for the pg_stat_statements query.
	pgStatQueryTimeout = 5 * time.Second

	pgStatSubsystem = "pgstat"
)

// Metric name constants for pg_stat_statements collector.
const (
	MetricPgStatQueryTotalTime = "platform_pgstat_query_total_time_ms"
	MetricPgStatQueryCalls     = "platform_pgstat_query_calls"
	MetricPgStatQueryMeanTime  = "platform_pgstat_query_mean_time_ms"
)

// Help string constants for pg_stat_statements collector.
const (
	helpPgStatQueryTotalTime = "Cumulative execution time in milliseconds for top-N queries by total_exec_time."
	helpPgStatQueryCalls     = "Total number of calls for top-N queries by total_exec_time."
	helpPgStatQueryMeanTime  = "Mean execution time in milliseconds for top-N queries by total_exec_time."
)

// Label constants for pg_stat_statements collector.
const (
	LabelQueryID = "queryid"
	LabelQuery   = "query"
)

// pgStatSelectQuery is the SQL query used to fetch top-N queries from
// pg_stat_statements ordered by total execution time descending.
const pgStatSelectQuery = `
	SELECT queryid::text,
	       left(regexp_replace(query, '\s+', ' ', 'g'), $1) AS query,
	       calls,
	       total_exec_time,
	       mean_exec_time
	FROM pg_stat_statements
	WHERE queryid IS NOT NULL
	ORDER BY total_exec_time DESC
	LIMIT $2
`

// PgStatCollector exposes pg_stat_statements top-N queries as Prometheus metrics.
// It implements prometheus.Collector and queries pg_stat_statements on each scrape.
//
// Metrics exported:
//   - platform_pgstat_query_total_time_ms{queryid, query} - cumulative execution time
//   - platform_pgstat_query_calls{queryid, query} - total number of calls
//   - platform_pgstat_query_mean_time_ms{queryid, query} - mean execution time
type PgStatCollector struct {
	pool   *pgxpool.Pool
	logger *slog.Logger

	totalTime *prometheus.Desc
	calls     *prometheus.Desc
	meanTime  *prometheus.Desc
}

// NewPgStatCollector returns a collector that exposes pg_stat_statements top-N
// query metrics. It must be registered with prometheus.Register.
func NewPgStatCollector(pool *pgxpool.Pool, logger *slog.Logger) *PgStatCollector {
	labels := []string{LabelQueryID, LabelQuery}

	return &PgStatCollector{
		pool:   pool,
		logger: logger,
		totalTime: prometheus.NewDesc(
			MetricPgStatQueryTotalTime,
			helpPgStatQueryTotalTime,
			labels, nil,
		),
		calls: prometheus.NewDesc(
			MetricPgStatQueryCalls,
			helpPgStatQueryCalls,
			labels, nil,
		),
		meanTime: prometheus.NewDesc(
			MetricPgStatQueryMeanTime,
			helpPgStatQueryMeanTime,
			labels, nil,
		),
	}
}

// Describe sends the metric descriptors to the channel.
func (c *PgStatCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.totalTime
	ch <- c.calls
	ch <- c.meanTime
}

// Collect fetches current top-N query stats from pg_stat_statements and sends
// them as Prometheus metrics. Errors are logged at debug level and do not cause
// scrape failures — the extension may not be available in all environments.
func (c *PgStatCollector) Collect(ch chan<- prometheus.Metric) {
	if c.pool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), pgStatQueryTimeout)
	defer cancel()

	rows, err := c.pool.Query(ctx, pgStatSelectQuery, pgStatQueryMaxLen, pgStatTopN)
	if err != nil {
		c.logger.Debug("pgstat collector: query failed", slog.Any("error", err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var queryID, query string
		var callCount int64
		var totalExecTime, meanExecTime float64

		if err := rows.Scan(&queryID, &query, &callCount, &totalExecTime, &meanExecTime); err != nil {
			c.logger.Debug("pgstat collector: row scan failed", slog.Any("error", err))
			continue
		}

		ch <- prometheus.MustNewConstMetric(c.totalTime, prometheus.GaugeValue, totalExecTime, queryID, query)
		ch <- prometheus.MustNewConstMetric(c.calls, prometheus.GaugeValue, float64(callCount), queryID, query)
		ch <- prometheus.MustNewConstMetric(c.meanTime, prometheus.GaugeValue, meanExecTime, queryID, query)
	}
}
