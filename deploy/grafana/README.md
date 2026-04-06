# Grafana Dashboard Panels

## Available Prometheus Metrics

### Outbox Relay
- `platform_outbox_relay_batch_size` -- histogram, labels: worker_id
- `platform_outbox_relay_batch_latency_seconds` -- histogram, labels: worker_id
- `platform_outbox_relay_publish_errors_total` -- counter
- `platform_outbox_relay_empty_polls_total` -- counter

### Event Consumer
- `platform_events_processed_total` -- counter, labels: event_type, status (success|failed|skipped_duplicate|dlq)
- `platform_event_processing_latency_seconds` -- histogram, labels: event_type
- `platform_entity_lock_wait_latency_seconds` -- histogram, labels: event_type
- `platform_dlq_published_total` -- counter
- `platform_idempotency_hit_total` -- counter, labels: event_type

### Circuit Breakers
- `platform_circuit_breaker_remnawave_state` -- gauge (0=closed, 1=half-open, 2=open)
- `platform_circuit_breaker_remnawave_transitions_total` -- counter, labels: to_state
- `platform_rate_limiter_fallback_total` -- counter

### Database (PostgreSQL)
- `platform_postgres_pool_connections` -- gauge
- `platform_postgres_pool_idle_connections` -- gauge
- `platform_postgres_pool_acquire_total` -- counter
- `platform_postgres_pool_acquire_duration_seconds_total` -- counter
- `platform_postgres_pool_max_connections` -- gauge

### Cache (Valkey)
- `platform_valkey_pool_hits_total` -- counter
- `platform_valkey_pool_misses_total` -- counter
- `platform_valkey_pool_timeouts_total` -- counter
- `platform_valkey_pool_connections` -- gauge
- `platform_valkey_pool_idle_connections` -- gauge

### NATS
- `platform_nats_messages_received_total` -- counter
- `platform_nats_messages_sent_total` -- counter
- `platform_nats_bytes_received_total` -- counter
- `platform_nats_bytes_sent_total` -- counter
- `platform_nats_reconnects_total` -- counter

### Suggested Alert Rules
- `increase(platform_dlq_published_total[5m]) > 0` -- DLQ activity
- `platform_circuit_breaker_remnawave_state == 2` -- Remnawave circuit open
- `platform_postgres_pool_connections >= platform_postgres_pool_max_connections * 0.9` -- Pool near exhaustion
