-- ============================================================================
-- Migration 037: Metrics time-series samples
-- ============================================================================
-- Periodic snapshots of DB-derivable platform metrics for time-series charts
-- (sparklines / trends on the admin dashboard). Written by the in-process
-- metrics collector (internal/domain/identity/service/metrics_collector.go).
--
-- This is a GLOBAL cross-tenant platform aggregate (same isolation model as
-- GET /api/admin/metrics and GET /api/admin/stats), so it has NO tenant_id
-- column and NO row-level security. Sourced ONLY from local tables; no
-- Remnawave data is captured here.
-- ============================================================================

CREATE SCHEMA IF NOT EXISTS metrics;

CREATE TABLE IF NOT EXISTS metrics.samples (
    id           UUID        PRIMARY KEY DEFAULT uuidv7(),
    captured_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    active_users BIGINT      NOT NULL DEFAULT 0,
    active_subs  BIGINT      NOT NULL DEFAULT 0,
    mrr_cents    BIGINT      NOT NULL DEFAULT 0,
    total_subs   BIGINT      NOT NULL DEFAULT 0,
    paying_users BIGINT      NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_metrics_samples_captured_at
    ON metrics.samples (captured_at DESC);
