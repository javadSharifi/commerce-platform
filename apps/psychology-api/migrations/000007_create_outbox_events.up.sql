-- 000007_create_outbox_events.up.sql
-- Transactional Outbox for Edge CDN Cache Invalidation and Asynchronous Events

CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    CONSTRAINT chk_outbox_status CHECK (status IN ('pending', 'processing', 'completed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_pending 
    ON outbox_events(status, created_at) 
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_outbox_events_tenant 
    ON outbox_events(tenant_id, status);

-- Enable and Force Row-Level Security (RLS)
ALTER TABLE outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox_events FORCE ROW LEVEL SECURITY;

-- Tenant Isolation Policy for Outbox Events
CREATE POLICY tenant_isolation_outbox_events ON outbox_events
    FOR ALL
    TO public
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
