-- 000002_create_workshops_and_payments.up.sql
-- Workshops, registrations, purchases, payment_attempts, and refunds

-- 1. Workshops Table (Tenant Scoped with RLS)
CREATE TABLE workshops (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title                 text        NOT NULL,
    slug                  text        NOT NULL,
    description           text        NOT NULL DEFAULT '',
    instructor_name       text        NOT NULL,
    starts_at             timestamptz NOT NULL,
    ends_at               timestamptz NOT NULL,
    location              text        NOT NULL,
    registration_deadline timestamptz NOT NULL,
    capacity              integer     NOT NULL CHECK (capacity >= 1),
    price_rials           bigint      NOT NULL DEFAULT 0 CHECK (price_rials >= 0),
    status                text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'closed', 'cancelled')),
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workshops_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT workshops_tenant_slug_uq UNIQUE (tenant_id, slug),
    CONSTRAINT workshops_dates_check CHECK (starts_at < ends_at AND registration_deadline <= starts_at)
);

CREATE INDEX workshops_tenant_status_idx ON workshops (tenant_id, status);
CREATE INDEX workshops_tenant_starts_at_idx ON workshops (tenant_id, starts_at);

ALTER TABLE workshops ENABLE ROW LEVEL SECURITY;
ALTER TABLE workshops FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON workshops
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 2. Purchases Table (Financial Snapshot & Orders)
CREATE TABLE purchases (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    item_type      text        NOT NULL CHECK (item_type IN ('workshop', 'course')),
    item_id        uuid        NOT NULL,
    amount_rials   bigint      NOT NULL CHECK (amount_rials > 0),
    title_snapshot text        NOT NULL,
    status         text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'refund_required', 'refunded', 'failed')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT purchases_tenant_id_id_uq UNIQUE (tenant_id, id)
);

CREATE INDEX purchases_tenant_status_idx ON purchases (tenant_id, status);
CREATE INDEX purchases_tenant_item_idx ON purchases (tenant_id, item_type, item_id);

ALTER TABLE purchases ENABLE ROW LEVEL SECURITY;
ALTER TABLE purchases FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON purchases
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 3. Workshop Registrations Table (Composite FK to workshops)
CREATE TABLE workshop_registrations (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workshop_id       uuid        NOT NULL,
    guest_name        text        NOT NULL,
    guest_phone       text        NOT NULL,
    status            text        NOT NULL DEFAULT 'pending_payment' CHECK (status IN ('pending_payment', 'confirmed', 'cancelled', 'expired')),
    expires_at        timestamptz,
    purchase_id       uuid,
    guest_lookup_hash text        NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT registrations_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT registrations_workshop_fk FOREIGN KEY (tenant_id, workshop_id) REFERENCES workshops(tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT registrations_purchase_fk FOREIGN KEY (tenant_id, purchase_id) REFERENCES purchases(tenant_id, id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX workshop_registrations_active_phone_uq
    ON workshop_registrations (tenant_id, workshop_id, guest_phone)
    WHERE status IN ('pending_payment', 'confirmed');

CREATE INDEX workshop_registrations_tenant_workshop_status_idx 
    ON workshop_registrations (tenant_id, workshop_id, status);

CREATE INDEX workshop_registrations_status_expires_at_idx 
    ON workshop_registrations (status, expires_at) 
    WHERE status = 'pending_payment';

CREATE INDEX workshop_registrations_lookup_hash_idx
    ON workshop_registrations (tenant_id, guest_lookup_hash);

ALTER TABLE workshop_registrations ENABLE ROW LEVEL SECURITY;
ALTER TABLE workshop_registrations FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON workshop_registrations
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 4. Payment Attempts Table (Composite FK to purchases)
CREATE TABLE payment_attempts (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    purchase_id     uuid        NOT NULL,
    provider        text        NOT NULL DEFAULT 'zarinpal' CHECK (provider IN ('zarinpal', 'mock')),
    authority       text        NOT NULL,
    amount_rials    bigint      NOT NULL CHECK (amount_rials > 0),
    status          text        NOT NULL DEFAULT 'initiated' CHECK (status IN ('initiated', 'verified', 'failed')),
    tracking_code   text,
    gateway_ref_id  text,
    idempotency_key text        NOT NULL,
    error_message   text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT payment_attempts_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT payment_attempts_purchase_fk FOREIGN KEY (tenant_id, purchase_id) REFERENCES purchases(tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT payment_attempts_tenant_authority_uq UNIQUE (tenant_id, authority),
    CONSTRAINT payment_attempts_tenant_idempotency_uq UNIQUE (tenant_id, idempotency_key)
);

CREATE INDEX payment_attempts_tenant_purchase_idx ON payment_attempts (tenant_id, purchase_id);

ALTER TABLE payment_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment_attempts FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON payment_attempts
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 5. Refunds Table (Composite FK to purchases)
CREATE TABLE refunds (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    purchase_id        uuid        NOT NULL,
    payment_attempt_id uuid,
    amount_rials       bigint      NOT NULL CHECK (amount_rials > 0),
    reason             text        NOT NULL,
    reference_number   text        NOT NULL,
    registered_by      uuid        NOT NULL REFERENCES panel_identities(id),
    registered_at      timestamptz NOT NULL DEFAULT now(),
    created_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT refunds_tenant_purchase_fk FOREIGN KEY (tenant_id, purchase_id) REFERENCES purchases(tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT refunds_tenant_reference_uq UNIQUE (tenant_id, reference_number)
);

CREATE INDEX refunds_tenant_purchase_idx ON refunds (tenant_id, purchase_id);

ALTER TABLE refunds ENABLE ROW LEVEL SECURITY;
ALTER TABLE refunds FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON refunds
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 6. Permissions for runtime app_user
GRANT SELECT, INSERT, UPDATE, DELETE ON workshops, workshop_registrations, purchases, payment_attempts, refunds TO app_user;
