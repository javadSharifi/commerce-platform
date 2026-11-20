-- 000004_create_media.up.sql
-- Media assets and uploads tracking with Tenant-scoped RLS and quota tracking

CREATE TABLE media_assets (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    storage_key       text        NOT NULL UNIQUE,
    original_name     text        NOT NULL,
    mime_type         text        NOT NULL,
    size_bytes        bigint      NOT NULL CHECK (size_bytes > 0),
    access_class      text        NOT NULL CHECK (access_class IN ('public', 'course_private')),
    validation_status text        NOT NULL DEFAULT 'pending' CHECK (validation_status IN ('pending', 'verified', 'rejected')),
    uploaded_by       uuid        NOT NULL REFERENCES panel_identities(id),
    verified_at       timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT media_assets_tenant_id_id_uq UNIQUE (tenant_id, id)
);

CREATE INDEX media_assets_tenant_validation_idx ON media_assets (tenant_id, validation_status);
CREATE INDEX media_assets_tenant_access_class_idx ON media_assets (tenant_id, access_class);

ALTER TABLE media_assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE media_assets FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON media_assets
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON media_assets TO app_user;

CREATE TABLE media_uploads (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    staging_key         text        NOT NULL UNIQUE,
    original_name       text        NOT NULL DEFAULT '',
    access_class        text        NOT NULL DEFAULT 'public' CHECK (access_class IN ('public', 'course_private')),
    expected_mime_type  text        NOT NULL,
    expected_size_bytes bigint      NOT NULL CHECK (expected_size_bytes > 0),
    expires_at          timestamptz NOT NULL,
    status              text        NOT NULL DEFAULT 'reserved' CHECK (status IN ('reserved', 'completed', 'expired')),
    initiated_by        uuid        NOT NULL REFERENCES panel_identities(id),
    created_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT media_uploads_tenant_id_id_uq UNIQUE (tenant_id, id)
);

CREATE INDEX media_uploads_tenant_status_idx ON media_uploads (tenant_id, status);
CREATE INDEX media_uploads_status_expires_at_idx ON media_uploads (status, expires_at) WHERE status = 'reserved';

ALTER TABLE media_uploads ENABLE ROW LEVEL SECURITY;
ALTER TABLE media_uploads FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON media_uploads
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON media_uploads TO app_user;
