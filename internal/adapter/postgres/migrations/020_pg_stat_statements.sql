-- ============================================================================
-- Migration 020: Enable pg_stat_statements extension
-- ============================================================================
-- Provides cumulative query execution statistics for performance analysis.
-- Requires shared_preload_libraries=pg_stat_statements in postgresql.conf
-- (or docker-compose command args).
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
