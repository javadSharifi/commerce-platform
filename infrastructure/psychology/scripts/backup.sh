#!/bin/sh
set -e

# infrastructure/psychology/scripts/backup.sh
# Automated PostgreSQL backup with timestamping and retention

BACKUP_DIR="${BACKUP_DIR:-/backups}"
POSTGRES_HOST="${POSTGRES_HOST:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-psychology_admin}"
POSTGRES_DB="${POSTGRES_DB:-psychology_prod}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"

mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="${BACKUP_DIR}/${POSTGRES_DB}_${TIMESTAMP}.dump"

echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Starting backup of ${POSTGRES_DB} to ${BACKUP_FILE}..."

pg_dump -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc -f "$BACKUP_FILE"

# Validate backup file exists and is non-empty
if [ ! -s "$BACKUP_FILE" ]; then
    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] ERROR: Backup file is empty or missing!" >&2
    exit 1
fi

BACKUP_SIZE=$(ls -lh "$BACKUP_FILE" | awk '{print $5}')
echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Backup completed successfully. Size: ${BACKUP_SIZE}"

# Clean up dumps older than RETENTION_DAYS
echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Cleaning up backups older than ${RETENTION_DAYS} days..."
find "$BACKUP_DIR" -type f -name "${POSTGRES_DB}_*.dump" -mtime +"$RETENTION_DAYS" -exec rm -f {} \; || true

echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Backup routine finished."
