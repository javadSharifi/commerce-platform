#!/bin/sh
set -e

# infrastructure/psychology/scripts/restore.sh
# Disaster recovery & database restore verification script

POSTGRES_HOST="${POSTGRES_HOST:-postgres}"
POSTGRES_USER="${POSTGRES_USER:-psychology_admin}"
POSTGRES_DB="${POSTGRES_DB:-psychology_prod}"
BACKUP_FILE="$1"

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <path_to_backup.dump>"
    exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
    echo "ERROR: Backup file $BACKUP_FILE does not exist!" >&2
    exit 1
fi

echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Restoring ${POSTGRES_DB} from ${BACKUP_FILE}..."

# Restore using pg_restore
pg_restore -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner --no-privileges "$BACKUP_FILE" || true

echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Verifying Row-Level Security (RLS) enforcement post-restore..."

# Query pg_tables to ensure all tenant-scoped tables have RLS enabled and forced
psql -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 << 'EOF'
DO $$
DECLARE
    r RECORD;
    unprotected_count INT := 0;
BEGIN
    FOR r IN (
        SELECT tablename
        FROM pg_tables
        WHERE schemaname = 'public'
          AND tablename IN ('workshops', 'workshop_registrations', 'purchases', 'payment_attempts', 'refunds', 'blog_posts', 'media_assets', 'media_uploads')
    ) LOOP
        IF NOT EXISTS (
            SELECT 1 FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'public' AND c.relname = r.tablename AND c.relrowsecurity = true AND c.relforcerowsecurity = true
        ) THEN
            RAISE WARNING 'Table % does NOT have forced RLS enabled!', r.tablename;
            unprotected_count := unprotected_count + 1;
        END IF;
    END LOOP;

    IF unprotected_count > 0 THEN
        RAISE EXCEPTION 'Disaster recovery failed: % tables lack forced RLS!', unprotected_count;
    END IF;
    RAISE NOTICE 'SUCCESS: All tenant-scoped tables have forced RLS verified.';
END $$;
EOF

echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] Restore and verification completed successfully."
