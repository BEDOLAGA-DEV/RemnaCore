#!/usr/bin/env bash
#
# Idempotent database migration runner.
#
# Applies every SQL file in the migrations directory exactly once, tracked in a
# public.schema_migrations ledger. Safe to run repeatedly: already-applied
# migrations are skipped, so adding a new NNN_*.sql file and re-running this
# script applies only the new one.
#
# Executor: psql (handles CREATE INDEX CONCURRENTLY, dollar-quoted functions and
# multi-statement files that an in-process runner cannot). The psql command is
# configurable via $PSQL so the same logic works locally and inside Docker:
#
#   local:   PSQL="psql $DATABASE_URL -v ON_ERROR_STOP=1" scripts/migrate.sh
#   docker:  PSQL="docker compose exec -T platform-db psql -U platform -d remnacore -v ON_ERROR_STOP=1" scripts/migrate.sh
#
# Existing databases (provisioned before this ledger existed) are detected via
# the multisub.saga_instances sentinel and baselined: every migration with a
# numeric prefix <= BASELINE_THROUGH is recorded as already applied WITHOUT
# re-running it (avoids "already exists" errors and duplicate data seeds), so
# only genuinely new migrations run. On a fresh database (no sentinel) nothing
# is baselined and all migrations run in order.
set -euo pipefail

MIGRATIONS_DIR="${MIGRATIONS_DIR:-internal/adapter/postgres/migrations}"
# Highest migration that shipped before the ledger was introduced. One-time
# adoption marker — irrelevant once the ledger is populated.
BASELINE_THROUGH="${BASELINE_THROUGH:-037}"
PSQL="${PSQL:-psql ${DATABASE_URL:?set DATABASE_URL or PSQL} -v ON_ERROR_STOP=1}"

q() { $PSQL -tAqc "$1" | tr -d '[:space:]'; }

# 1. Ensure the ledger exists.
q "CREATE TABLE IF NOT EXISTS public.schema_migrations (
     version    TEXT        PRIMARY KEY,
     applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
   );" >/dev/null

# 2. One-time baseline for an existing (pre-ledger) database.
ledger_count="$(q "SELECT count(*) FROM public.schema_migrations")"
sentinel="$(q "SELECT 1 FROM information_schema.tables WHERE table_schema='multisub' AND table_name='saga_instances'")"
if [ "$ledger_count" = "0" ] && [ "$sentinel" = "1" ]; then
  echo "Existing database detected — baselining migrations through ${BASELINE_THROUGH} (not re-running them)."
  for f in "$MIGRATIONS_DIR"/*.sql; do
    version="$(basename "$f")"
    prefix="${version%%_*}"
    if [ "$((10#$prefix))" -le "$((10#$BASELINE_THROUGH))" ]; then
      q "INSERT INTO public.schema_migrations (version) VALUES ('$version') ON CONFLICT DO NOTHING;" >/dev/null
    fi
  done
fi

# 3. Apply any migration not yet in the ledger, in filename order.
applied=0
for f in "$MIGRATIONS_DIR"/*.sql; do
  version="$(basename "$f")"
  if [ "$(q "SELECT 1 FROM public.schema_migrations WHERE version='$version'")" = "1" ]; then
    continue
  fi
  echo "  applying ${version} ..."
  $PSQL < "$f" >/dev/null
  q "INSERT INTO public.schema_migrations (version) VALUES ('$version') ON CONFLICT DO NOTHING;" >/dev/null
  applied=$((applied + 1))
done

echo "Migrations up to date (${applied} applied this run)."
