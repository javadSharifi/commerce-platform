-- 000006_create_courses.up.sql
-- Recorded Courses, Lessons, Buyer Accounts, Sessions, OTP, and Entitlements

-- 1. buyer_accounts (Student/Buyer identity scoped to Tenant)
CREATE TABLE buyer_accounts (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    phone      text        NOT NULL,
    full_name  text        NOT NULL DEFAULT '',
    status     text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT buyer_accounts_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT buyer_accounts_tenant_phone_uq UNIQUE (tenant_id, phone)
);

ALTER TABLE buyer_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE buyer_accounts FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON buyer_accounts
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE INDEX idx_buyer_accounts_tenant_phone ON buyer_accounts (tenant_id, phone);

-- 2. buyer_sessions (Server-side sessions for authenticated buyers)
CREATE TABLE buyer_sessions (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    buyer_id   uuid        NOT NULL,
    token_hash text        NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT buyer_sessions_tenant_buyer_fk FOREIGN KEY (tenant_id, buyer_id) REFERENCES buyer_accounts(tenant_id, id) ON DELETE CASCADE
);

ALTER TABLE buyer_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE buyer_sessions FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON buyer_sessions
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE INDEX idx_buyer_sessions_tenant_buyer ON buyer_sessions (tenant_id, buyer_id);
CREATE INDEX idx_buyer_sessions_expires_at ON buyer_sessions (expires_at);

-- 3. buyer_otp_codes (Short-lived verification codes for login/register)
CREATE TABLE buyer_otp_codes (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    phone       text        NOT NULL,
    code_hash   text        NOT NULL,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    attempts    int         NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE buyer_otp_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE buyer_otp_codes FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON buyer_otp_codes
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE INDEX idx_buyer_otp_codes_tenant_phone ON buyer_otp_codes (tenant_id, phone);

-- 4. courses (Recorded courses catalog)
CREATE TABLE courses (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title           text        NOT NULL,
    slug            text        NOT NULL,
    description     text        NOT NULL DEFAULT '',
    instructor_name text        NOT NULL,
    price_rials     bigint      NOT NULL CHECK (price_rials >= 0),
    cover_media_id  uuid,
    status          text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    published_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,
    CONSTRAINT courses_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT courses_tenant_slug_uq UNIQUE (tenant_id, slug)
);

ALTER TABLE courses ENABLE ROW LEVEL SECURITY;
ALTER TABLE courses FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON courses
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE INDEX idx_courses_tenant_status ON courses (tenant_id, status) WHERE (deleted_at IS NULL);

-- 5. course_lessons (Syllabus items / sessions with private media files)
CREATE TABLE course_lessons (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    course_id        uuid        NOT NULL,
    title            text        NOT NULL,
    description      text        NOT NULL DEFAULT '',
    display_order    int         NOT NULL DEFAULT 1,
    media_asset_id   uuid        NOT NULL,
    duration_seconds int         NOT NULL DEFAULT 0 CHECK (duration_seconds >= 0),
    is_free_preview  boolean     NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz,
    CONSTRAINT course_lessons_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT course_lessons_tenant_course_fk FOREIGN KEY (tenant_id, course_id) REFERENCES courses(tenant_id, id) ON DELETE CASCADE
);

ALTER TABLE course_lessons ENABLE ROW LEVEL SECURITY;
ALTER TABLE course_lessons FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON course_lessons
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

CREATE INDEX idx_course_lessons_order ON course_lessons (tenant_id, course_id, display_order) WHERE (deleted_at IS NULL);

-- 6. course_entitlements (Purchased or granted access to courses)
CREATE TABLE course_entitlements (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    buyer_id              uuid        NOT NULL,
    course_id             uuid        NOT NULL,
    purchase_id           uuid,
    status                text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'revoked')),
    course_title_snapshot text        NOT NULL,
    price_rials_snapshot  bigint      NOT NULL CHECK (price_rials_snapshot >= 0),
    granted_at            timestamptz,
    revoked_at            timestamptz,
    revocation_reason     text,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT course_entitlements_tenant_buyer_fk FOREIGN KEY (tenant_id, buyer_id) REFERENCES buyer_accounts(tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT course_entitlements_tenant_course_fk FOREIGN KEY (tenant_id, course_id) REFERENCES courses(tenant_id, id) ON DELETE RESTRICT
);

ALTER TABLE course_entitlements ENABLE ROW LEVEL SECURITY;
ALTER TABLE course_entitlements FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON course_entitlements
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- Crucial: Guarantee exactly ONE active entitlement per student per course
CREATE UNIQUE INDEX course_entitlements_active_uq ON course_entitlements (tenant_id, buyer_id, course_id) WHERE (status = 'active');
CREATE INDEX idx_course_entitlements_buyer ON course_entitlements (tenant_id, buyer_id, status);

-- Grants
GRANT SELECT, INSERT, UPDATE, DELETE ON buyer_accounts, buyer_sessions, buyer_otp_codes, courses, course_lessons, course_entitlements TO app_user;
