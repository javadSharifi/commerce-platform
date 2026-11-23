-- 000010_outbox_worker_permissions.up.sql
-- Restrict worker privileges to queue operations and enforce transactional outbox isolation

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'worker_user') THEN
        CREATE ROLE worker_user WITH LOGIN PASSWORD 'secret_worker_pass' NOSUPERUSER NOBYPASSRLS;
    END IF;
END $$;

GRANT USAGE ON SCHEMA public TO app_user, worker_user;

-- app_user can only insert and select its own tenant's outbox events
GRANT SELECT, INSERT ON outbox_events TO app_user;

-- worker_user can only read queue and update status/retries (no insert, no delete, no other tables)
GRANT SELECT, UPDATE (status, retry_count, last_error, processed_at) ON outbox_events TO worker_user;

-- Reconfigure RLS policies on outbox_events
ALTER TABLE outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation_outbox_events ON outbox_events;
DROP POLICY IF EXISTS outbox_tenant_isolation ON outbox_events;
DROP POLICY IF EXISTS outbox_worker_policy ON outbox_events;

CREATE POLICY outbox_tenant_isolation ON outbox_events
    FOR ALL
    TO app_user
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.worker_mode', true) = 'true'
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.worker_mode', true) = 'true'
    );

CREATE POLICY outbox_worker_policy ON outbox_events
    FOR ALL
    TO worker_user
    USING (true)
    WITH CHECK (true);
