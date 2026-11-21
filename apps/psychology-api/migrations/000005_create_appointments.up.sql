-- 000005_create_appointments.up.sql
-- Therapists, therapist working hours, schedule exceptions, appointment slots, appointments, and status history with Tenant RLS

-- 1. Therapists Table (Tenant Scoped with RLS)
CREATE TABLE therapists (
    id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id                  uuid        REFERENCES panel_identities(id) ON DELETE SET NULL,
    name                     text        NOT NULL,
    slug                     text        NOT NULL,
    title                    text        NOT NULL DEFAULT '',
    bio                      text        NOT NULL DEFAULT '',
    avatar_asset_id          uuid        REFERENCES media_assets(id) ON DELETE SET NULL,
    session_duration_minutes integer     NOT NULL DEFAULT 45 CHECK (session_duration_minutes > 0),
    is_active                boolean     NOT NULL DEFAULT true,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    deleted_at               timestamptz,
    CONSTRAINT therapists_tenant_id_id_uq UNIQUE (tenant_id, id)
);

CREATE UNIQUE INDEX therapists_tenant_slug_uq ON therapists (tenant_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX therapists_tenant_is_active_idx ON therapists (tenant_id, is_active) WHERE deleted_at IS NULL;
CREATE INDEX therapists_user_id_idx ON therapists (user_id);

ALTER TABLE therapists ENABLE ROW LEVEL SECURITY;
ALTER TABLE therapists FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON therapists
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 2. Therapist Working Hours Table (Weekly recurring slots, 0=Saturday to 6=Friday)
CREATE TABLE therapist_working_hours (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    therapist_id uuid        NOT NULL,
    day_of_week  smallint    NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    start_time   time        NOT NULL,
    end_time     time        NOT NULL,
    is_active    boolean     NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT therapist_working_hours_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT therapist_working_hours_therapist_fk FOREIGN KEY (tenant_id, therapist_id) REFERENCES therapists(tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT therapist_working_hours_time_check CHECK (start_time < end_time)
);

CREATE INDEX therapist_working_hours_lookup_idx ON therapist_working_hours (tenant_id, therapist_id, day_of_week);

ALTER TABLE therapist_working_hours ENABLE ROW LEVEL SECURITY;
ALTER TABLE therapist_working_hours FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON therapist_working_hours
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 3. Therapist Schedule Exceptions Table (Holidays, time off, or specific day blocks)
CREATE TABLE therapist_schedule_exceptions (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    therapist_id   uuid        NOT NULL,
    date           date        NOT NULL,
    is_unavailable boolean     NOT NULL DEFAULT true,
    start_time     time,
    end_time       time,
    reason         text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT therapist_exceptions_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT therapist_exceptions_therapist_fk FOREIGN KEY (tenant_id, therapist_id) REFERENCES therapists(tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT therapist_exceptions_time_check CHECK (
        (is_unavailable = true AND start_time IS NULL AND end_time IS NULL) OR
        (start_time IS NOT NULL AND end_time IS NOT NULL AND start_time < end_time)
    )
);

CREATE INDEX therapist_exceptions_date_idx ON therapist_schedule_exceptions (tenant_id, therapist_id, date);

ALTER TABLE therapist_schedule_exceptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE therapist_schedule_exceptions FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON therapist_schedule_exceptions
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 4. Appointment Slots Table (Generated individual session slots)
CREATE TABLE appointment_slots (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    therapist_id uuid        NOT NULL,
    starts_at    timestamptz NOT NULL,
    ends_at      timestamptz NOT NULL,
    status       text        NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'reserved', 'booked', 'blocked')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT appointment_slots_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT appointment_slots_therapist_fk FOREIGN KEY (tenant_id, therapist_id) REFERENCES therapists(tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT appointment_slots_time_check CHECK (starts_at < ends_at),
    CONSTRAINT appointment_slots_unique_time UNIQUE (tenant_id, therapist_id, starts_at)
);

CREATE INDEX appointment_slots_tenant_therapist_idx ON appointment_slots (tenant_id, therapist_id, starts_at);
CREATE INDEX appointment_slots_tenant_status_idx ON appointment_slots (tenant_id, status);

ALTER TABLE appointment_slots ENABLE ROW LEVEL SECURITY;
ALTER TABLE appointment_slots FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON appointment_slots
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 5. Appointments Table (Bookings with guest lookup hash & status cycle)
CREATE TABLE appointments (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    therapist_id        uuid        NOT NULL,
    slot_id             uuid        NOT NULL,
    status              text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'cancelled', 'expired', 'completed')),
    guest_name          text        NOT NULL,
    guest_phone         text        NOT NULL,
    guest_lookup_hash   text        NOT NULL,
    notes               text        NOT NULL DEFAULT '',
    expires_at          timestamptz NOT NULL,
    confirmed_at        timestamptz,
    cancelled_at        timestamptz,
    cancellation_reason text        NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT appointments_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT appointments_therapist_fk FOREIGN KEY (tenant_id, therapist_id) REFERENCES therapists(tenant_id, id) ON DELETE RESTRICT,
    CONSTRAINT appointments_slot_fk FOREIGN KEY (tenant_id, slot_id) REFERENCES appointment_slots(tenant_id, id) ON DELETE RESTRICT
);

-- At most one active booking per slot per tenant
CREATE UNIQUE INDEX appointments_active_slot_uq
    ON appointments (tenant_id, slot_id)
    WHERE status IN ('pending', 'confirmed', 'completed');

CREATE INDEX appointments_tenant_status_idx ON appointments (tenant_id, status);
CREATE INDEX appointments_tenant_therapist_idx ON appointments (tenant_id, therapist_id, created_at);
CREATE INDEX appointments_expires_at_idx ON appointments (expires_at) WHERE status = 'pending';
CREATE INDEX appointments_lookup_hash_idx ON appointments (tenant_id, guest_lookup_hash);

ALTER TABLE appointments ENABLE ROW LEVEL SECURITY;
ALTER TABLE appointments FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON appointments
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 6. Appointment Status History Table (Audit log for booking status transitions)
CREATE TABLE appointment_status_history (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    appointment_id uuid        NOT NULL,
    from_status    text,
    to_status      text        NOT NULL,
    action         text        NOT NULL CHECK (action IN ('created', 'confirmed', 'cancelled', 'rescheduled', 'expired', 'completed')),
    actor_type     text        NOT NULL CHECK (actor_type IN ('guest', 'staff', 'system')),
    actor_id       uuid        REFERENCES panel_identities(id) ON DELETE SET NULL,
    reason         text        NOT NULL DEFAULT '',
    metadata       jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT appointment_history_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT appointment_history_appointment_fk FOREIGN KEY (tenant_id, appointment_id) REFERENCES appointments(tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX appointment_history_appointment_idx ON appointment_status_history (tenant_id, appointment_id, created_at);

ALTER TABLE appointment_status_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE appointment_status_history FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON appointment_status_history
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

-- 7. Grant Permissions to app_user
GRANT SELECT, INSERT, UPDATE, DELETE ON therapists TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON therapist_working_hours TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON therapist_schedule_exceptions TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON appointment_slots TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON appointments TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON appointment_status_history TO app_user;
