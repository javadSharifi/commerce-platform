-- 000001_create_core_schema.up.sql
-- Core tenants, domains, site_settings, panel_identities, memberships, panel_sessions, password_resets

-- 1. Tenants Table
CREATE TABLE tenants (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL,
    slug       text        NOT NULL UNIQUE,
    status     text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- 3. Domains Table (Tenant Scoped with RLS)
CREATE TABLE domains (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    host        text        NOT NULL UNIQUE,
    is_primary  boolean     NOT NULL DEFAULT false,
    is_verified boolean     NOT NULL DEFAULT false,
    verified_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX domains_tenant_id_idx ON domains (tenant_id);

ALTER TABLE domains ENABLE ROW LEVEL SECURITY;
ALTER TABLE domains FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON domains
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 4. Site Settings Table (Tenant Scoped with RLS)
CREATE TABLE site_settings (
    tenant_id     uuid        PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    title         text        NOT NULL DEFAULT '',
    description   text        NOT NULL DEFAULT '',
    phone         text        NOT NULL DEFAULT '',
    address       text        NOT NULL DEFAULT '',
    primary_color text        NOT NULL DEFAULT '#0284c7',
    logo_media_id uuid,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE site_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE site_settings FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON site_settings
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 5. Panel Identities (Users) Table
CREATE TABLE panel_identities (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL UNIQUE,
    password_hash text        NOT NULL,
    full_name     text        NOT NULL,
    status        text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- 6. Memberships Table (Tenant Scoped with RLS)
CREATE TABLE memberships (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL REFERENCES panel_identities(id) ON DELETE CASCADE,
    role       text        NOT NULL CHECK (role IN ('owner', 'content_manager', 'receptionist', 'therapist')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT memberships_tenant_user_uq UNIQUE (tenant_id, user_id)
);

CREATE INDEX memberships_tenant_id_idx ON memberships (tenant_id);
CREATE INDEX memberships_user_id_idx ON memberships (user_id);

ALTER TABLE memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE memberships FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON memberships
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 7. Panel Sessions Table
CREATE TABLE panel_sessions (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES panel_identities(id) ON DELETE CASCADE,
    token_hash text        NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX panel_sessions_token_hash_idx ON panel_sessions (token_hash);
CREATE INDEX panel_sessions_expires_at_idx ON panel_sessions (expires_at);

-- 8. Password Resets Table
CREATE TABLE password_resets (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES panel_identities(id) ON DELETE CASCADE,
    code_hash  text        NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX password_resets_user_id_idx ON password_resets (user_id);

-- 9. Runtime Permissions
GRANT USAGE ON SCHEMA public TO app_user, lookup_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
GRANT SELECT ON domains, tenants, panel_identities, memberships TO lookup_user;
