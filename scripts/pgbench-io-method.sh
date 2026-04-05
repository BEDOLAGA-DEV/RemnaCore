#!/usr/bin/env bash
# Compare PostgreSQL io_method=worker vs io_method=io_uring performance.
# Requires: pgbench (from postgresql-client), psql, running PG instance.
#
# Usage: ./scripts/pgbench-io-method.sh [database_url]
#
# Results are printed to stdout. Run twice with different PG configs:
#   docker-compose up -d platform-db  # with io_method=worker (default)
#   ./scripts/pgbench-io-method.sh
#   # Then change to io_method=io_uring in docker-compose.yml and restart
#   ./scripts/pgbench-io-method.sh

set -euo pipefail

# Benchmark tuning constants.
readonly CLIENTS=10
readonly THREADS=4
readonly DURATION_SECONDS=30

DB_URL="${PGBENCH_DATABASE_URL:-${1:?Usage: $0 <database_url> or set PGBENCH_DATABASE_URL}}"

echo "=== pgbench I/O Method Benchmark ==="
echo "Database: ${DB_URL%%@*}@***"
echo "Clients: ${CLIENTS}  Threads: ${THREADS}  Duration: ${DURATION_SECONDS}s"
echo ""

echo "=== Initializing pgbench tables ==="
pgbench -i "$DB_URL"

echo ""
echo "=== Read-heavy workload (SELECT-only) ==="
pgbench -c "$CLIENTS" -j "$THREADS" -T "$DURATION_SECONDS" -S "$DB_URL"

echo ""
echo "=== Mixed workload (TPC-B) ==="
pgbench -c "$CLIENTS" -j "$THREADS" -T "$DURATION_SECONDS" "$DB_URL"

echo ""
echo "=== Write-heavy workload (simple-update) ==="
pgbench -c "$CLIENTS" -j "$THREADS" -T "$DURATION_SECONDS" -N "$DB_URL"

echo ""
echo "=== Current PG settings ==="
psql "$DB_URL" -c "SHOW io_method; SHOW effective_io_concurrency; SHOW shared_buffers; SHOW work_mem;"
