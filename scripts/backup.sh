#!/usr/bin/env bash
#
# Postgres backup for RemnaCore (platform + Remnawave databases).
#
# All durable state lives in the platform_pgdata and remnawave_pgdata volumes;
# nothing else backs them up. A single `docker volume rm`, disk failure, or bad
# migration is otherwise total, unrecoverable loss of billing/tenant data.
#
# Produces per-database, timestamped, gzip'd pg_dump files (custom format, so
# they restore with pg_restore) under $BACKUP_DIR, and prunes dumps older than
# $BACKUP_RETENTION_DAYS.
#
# Usage:
#   ./scripts/backup.sh                       # dump both DBs to ./backups
#   BACKUP_DIR=/mnt/backups ./scripts/backup.sh
#
# Cron (daily at 03:30, log to syslog):
#   30 3 * * * cd /root/RemnaCore && ./scripts/backup.sh >> /var/log/remnacore-backup.log 2>&1
#
# Restore (example, platform DB):
#   gunzip -c backups/platform-YYYYmmdd-HHMMSS.dump.gz \
#     | docker compose exec -T platform-db pg_restore -U platform -d remnacore --clean --if-exists
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

BACKUP_DIR="${BACKUP_DIR:-$PROJECT_DIR/backups}"
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
PLATFORM_DB_USER="${PLATFORM_DB_USER:-platform}"
PLATFORM_DB_NAME="${PLATFORM_DB_NAME:-remnacore}"
REMNAWAVE_DB_USER="${REMNAWAVE_DB_USER:-remnawave}"
REMNAWAVE_DB_NAME="${REMNAWAVE_DB_NAME:-remnawave}"

# Load .env so credentials/names match the running stack.
if [ -f .env ]; then
    set -a
    # shellcheck disable=SC1091
    . ./.env
    set +a
fi

mkdir -p "$BACKUP_DIR"
stamp="$(date -u +%Y%m%d-%H%M%S)"

# dump SERVICE USER DBNAME OUTFILE — pg_dump custom format, piped to gzip.
dump() {
    local service="$1" user="$2" db="$3" out="$4"
    echo "  dumping ${service} (${db}) → $(basename "$out")"
    if docker compose exec -T "$service" pg_dump -U "$user" -d "$db" -F c 2>/dev/null | gzip > "$out"; then
        # A valid gzip'd custom dump is never near-empty; guard against a silent
        # failure that produced a truncated file.
        if [ "$(stat -f%z "$out" 2>/dev/null || stat -c%s "$out" 2>/dev/null || echo 0)" -lt 100 ]; then
            echo "  ERROR: ${service} dump is suspiciously small — treating as failure" >&2
            rm -f "$out"
            return 1
        fi
    else
        echo "  ERROR: ${service} dump failed" >&2
        rm -f "$out"
        return 1
    fi
}

rc=0
dump platform-db "$PLATFORM_DB_USER" "$PLATFORM_DB_NAME" "$BACKUP_DIR/platform-${stamp}.dump.gz" || rc=1
dump remnawave-db "$REMNAWAVE_DB_USER" "$REMNAWAVE_DB_NAME" "$BACKUP_DIR/remnawave-${stamp}.dump.gz" || rc=1

# Prune old dumps (only on a fully successful run, so a failed backup never
# deletes the last good one).
if [ "$rc" -eq 0 ]; then
    find "$BACKUP_DIR" -name '*.dump.gz' -type f -mtime "+${BACKUP_RETENTION_DAYS}" -print -delete || true
    echo "[OK] Backup complete → $BACKUP_DIR (retention ${BACKUP_RETENTION_DAYS}d)"
else
    echo "[FAIL] One or more dumps failed; old backups were NOT pruned" >&2
fi
exit "$rc"
