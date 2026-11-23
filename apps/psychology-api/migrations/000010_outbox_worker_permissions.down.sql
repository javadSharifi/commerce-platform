-- 000010_outbox_worker_permissions.down.sql
-- Revert worker privileges and restore initial outbox policy

DROP POLICY IF EXISTS outbox_worker_policy ON outbox_events;
DROP POLICY IF EXISTS outbox_tenant_isolation ON outbox_events;

CREATE POLICY tenant_isolation_outbox_events ON outbox_events
    FOR ALL
    TO public
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

REVOKE ALL ON outbox_events FROM worker_user;
