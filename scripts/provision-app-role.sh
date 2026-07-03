#!/usr/bin/env bash
#
# Provision the least-privilege application database role.
#
# The application MUST NOT connect as a superuser or a BYPASSRLS role: either
# would silently bypass FORCE ROW LEVEL SECURITY and void tenant isolation, so
# the app asserts this at boot and refuses to start (internal/adapter/postgres/
# boot_assert.go). Migrations, however, still run as the bootstrap superuser
# (they need CREATE EXTENSION etc.). This script bridges the two: run as the
# superuser AFTER migrations, it creates/updates a NOSUPERUSER NOBYPASSRLS role
# and grants it exactly the privileges the running app needs.
#
# Idempotent and safe to re-run on every deploy (it re-grants across all current
# schemas, picking up tables added by new migrations).
#
# Config (same $PSQL indirection as scripts/migrate.sh so it works locally and
# in Docker):
#   local:   PSQL="psql $SUPERUSER_URL -v ON_ERROR_STOP=1" APP_DB_USER=remnacore_app APP_DB_PASSWORD=... scripts/provision-app-role.sh
#   docker:  PSQL="docker compose exec -T platform-db psql -U platform -d remnacore -v ON_ERROR_STOP=1" APP_DB_USER=... APP_DB_PASSWORD=... scripts/provision-app-role.sh
set -euo pipefail

APP_DB_USER="${APP_DB_USER:-remnacore_app}"
APP_DB_PASSWORD="${APP_DB_PASSWORD:?APP_DB_PASSWORD must be set}"
PSQL="${PSQL:-psql ${SUPERUSER_URL:?set SUPERUSER_URL or PSQL} -v ON_ERROR_STOP=1}"

# The role name is a fixed identifier (never user-controlled), so it is embedded
# directly; only the password is passed as a psql variable (safely quoted with
# %L / :'app_pass' outside any dollar-quoted block).
if [ "$APP_DB_USER" != "remnacore_app" ]; then
  echo "provision-app-role: APP_DB_USER must be 'remnacore_app' (got '$APP_DB_USER')" >&2
  exit 1
fi

$PSQL -v app_pass="$APP_DB_PASSWORD" <<'SQL'
-- 1. Role: create if missing (LOGIN, explicitly NOSUPERUSER + NOBYPASSRLS so the
--    boot assertion passes), then always (re)set the password idempotently.
SELECT format(
    'CREATE ROLE remnacore_app LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE INHERIT PASSWORD %L',
    :'app_pass')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'remnacore_app')
\gexec

ALTER ROLE remnacore_app LOGIN NOSUPERUSER NOBYPASSRLS PASSWORD :'app_pass';

-- 2. Grants across every application schema (dynamic so new schemas from future
--    migrations are covered on the next run). GRANT ... ON ALL covers existing
--    objects; ALTER DEFAULT PRIVILEGES covers objects future migrations create
--    (they run as this same superuser).
DO $$
DECLARE s text;
BEGIN
    FOR s IN
        SELECT nspname FROM pg_namespace
        WHERE nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
          AND nspname NOT LIKE 'pg_temp%'
          AND nspname NOT LIKE 'pg_toast_temp%'
    LOOP
        EXECUTE format('GRANT USAGE ON SCHEMA %I TO remnacore_app', s);
        EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %I TO remnacore_app', s);
        EXECUTE format('GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA %I TO remnacore_app', s);
        EXECUTE format('GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA %I TO remnacore_app', s);
        EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO remnacore_app', s);
        EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO remnacore_app', s);
        EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT EXECUTE ON FUNCTIONS TO remnacore_app', s);
    END LOOP;
END $$;

-- 3. Runtime partition management: the PartitionManager creates and drops
--    public.outbox partitions at runtime (CREATE TABLE ... PARTITION OF,
--    DETACH, DROP), which requires ownership of the parent and every partition
--    plus CREATE on the schema. Transfer ownership of the outbox table family
--    to the app role (outbox is infrastructure, not tenant-scoped).
DO $$
DECLARE t text;
BEGIN
    FOR t IN
        SELECT n.nspname || '.' || quote_ident(c.relname)
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relkind IN ('r', 'p')            -- ordinary + partitioned tables
          AND c.relname LIKE 'outbox%'           -- parent + outbox_* partitions
    LOOP
        EXECUTE format('ALTER TABLE %s OWNER TO remnacore_app', t);
    END LOOP;
END $$;

GRANT CREATE ON SCHEMA public TO remnacore_app;

-- 4. Observability: let the pg_stat_statements metrics collector read all
--    statements. pg_read_all_stats is a predefined monitoring role WITHOUT
--    BYPASSRLS, so the boot assertion still passes (it only rejects superuser /
--    BYPASSRLS). Harmless if pg_stat_statements is absent.
GRANT pg_read_all_stats TO remnacore_app;
SQL

echo "provision-app-role: remnacore_app provisioned (NOSUPERUSER NOBYPASSRLS)."
