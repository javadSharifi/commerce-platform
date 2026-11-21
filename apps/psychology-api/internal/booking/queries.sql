-- Therapist queries
-- name: CreateTherapist :one
INSERT INTO therapists (
    tenant_id,
    user_id,
    name,
    slug,
    title,
    bio,
    avatar_asset_id,
    session_duration_minutes,
    is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetTherapistByID :one
SELECT * FROM therapists
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: GetTherapistBySlug :one
SELECT * FROM therapists
WHERE tenant_id = $1 AND slug = $2 AND deleted_at IS NULL;

-- name: GetTherapistByUserID :one
SELECT * FROM therapists
WHERE tenant_id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: ListTherapists :many
SELECT * FROM therapists
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY created_at ASC;

-- name: ListActiveTherapists :many
SELECT * FROM therapists
WHERE tenant_id = $1 AND is_active = true AND deleted_at IS NULL
ORDER BY name ASC;

-- name: UpdateTherapist :one
UPDATE therapists
SET user_id = $3,
    name = $4,
    slug = $5,
    title = $6,
    bio = $7,
    avatar_asset_id = $8,
    session_duration_minutes = $9,
    is_active = $10,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteTherapist :exec
UPDATE therapists
SET deleted_at = now(),
    updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- Working hours queries
-- name: ListWorkingHoursByTherapist :many
SELECT * FROM therapist_working_hours
WHERE tenant_id = $1 AND therapist_id = $2
ORDER BY day_of_week ASC, start_time ASC;

-- name: DeleteWorkingHoursByTherapist :exec
DELETE FROM therapist_working_hours
WHERE tenant_id = $1 AND therapist_id = $2;

-- name: CreateWorkingHour :one
INSERT INTO therapist_working_hours (
    tenant_id,
    therapist_id,
    day_of_week,
    start_time,
    end_time,
    is_active
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- Schedule exceptions queries
-- name: ListScheduleExceptionsByTherapist :many
SELECT * FROM therapist_schedule_exceptions
WHERE tenant_id = $1 AND therapist_id = $2
ORDER BY date ASC, start_time ASC;

-- name: GetScheduleExceptionByID :one
SELECT * FROM therapist_schedule_exceptions
WHERE tenant_id = $1 AND id = $2;

-- name: CreateScheduleException :one
INSERT INTO therapist_schedule_exceptions (
    tenant_id,
    therapist_id,
    date,
    is_unavailable,
    start_time,
    end_time,
    reason
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: DeleteScheduleException :exec
DELETE FROM therapist_schedule_exceptions
WHERE tenant_id = $1 AND id = $2;

-- Appointment slots queries
-- name: CreateAppointmentSlot :one
INSERT INTO appointment_slots (
    tenant_id,
    therapist_id,
    starts_at,
    ends_at,
    status
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetAppointmentSlotByID :one
SELECT * FROM appointment_slots
WHERE tenant_id = $1 AND id = $2;

-- name: GetAppointmentSlotByIDForUpdate :one
SELECT * FROM appointment_slots
WHERE tenant_id = $1 AND id = $2
FOR UPDATE;

-- name: ListSlotsByTherapist :many
SELECT * FROM appointment_slots
WHERE tenant_id = $1 AND therapist_id = $2
  AND starts_at >= $3 AND ends_at <= $4
ORDER BY starts_at ASC;

-- name: ListAvailableSlots :many
SELECT * FROM appointment_slots
WHERE tenant_id = $1 AND therapist_id = $2
  AND status = 'available'
  AND starts_at >= $3 AND starts_at <= $4
ORDER BY starts_at ASC;

-- name: DeleteFutureAvailableSlots :exec
DELETE FROM appointment_slots
WHERE tenant_id = $1 AND therapist_id = $2
  AND status = 'available'
  AND starts_at >= $3;

-- name: UpdateSlotStatus :one
UPDATE appointment_slots
SET status = $3,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- Appointments queries
-- name: CreateAppointment :one
INSERT INTO appointments (
    tenant_id,
    therapist_id,
    slot_id,
    status,
    guest_name,
    guest_phone,
    guest_lookup_hash,
    notes,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetAppointmentByID :one
SELECT * FROM appointments
WHERE tenant_id = $1 AND id = $2;

-- name: GetAppointmentByIDForUpdate :one
SELECT * FROM appointments
WHERE tenant_id = $1 AND id = $2
FOR UPDATE;

-- name: GetAppointmentBySlotID :one
SELECT * FROM appointments
WHERE tenant_id = $1 AND slot_id = $2
  AND status IN ('pending', 'confirmed', 'completed');

-- name: UpdateAppointmentStatus :one
UPDATE appointments
SET status = $3,
    confirmed_at = CASE WHEN $3 = 'confirmed' THEN now() ELSE confirmed_at END,
    cancelled_at = CASE WHEN $3 = 'cancelled' THEN now() ELSE cancelled_at END,
    cancellation_reason = CASE WHEN $3 = 'cancelled' THEN $4 ELSE cancellation_reason END,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- name: ListAppointmentsByTherapist :many
SELECT * FROM appointments
WHERE tenant_id = $1 AND therapist_id = $2
ORDER BY created_at DESC;

-- name: ListExpiredPendingAppointments :many
SELECT * FROM appointments
WHERE tenant_id = $1 AND status = 'pending' AND expires_at <= $2;

-- Appointment Status History queries
-- name: CreateAppointmentStatusHistory :one
INSERT INTO appointment_status_history (
    tenant_id,
    appointment_id,
    from_status,
    to_status,
    action,
    actor_type,
    actor_id,
    reason,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: ListAppointmentHistory :many
SELECT * FROM appointment_status_history
WHERE tenant_id = $1 AND appointment_id = $2
ORDER BY created_at ASC;

-- name: GetAppointmentDetail :one
SELECT a.*, t.name as therapist_name, s.starts_at, s.ends_at
FROM appointments a
JOIN therapists t ON t.tenant_id = a.tenant_id AND t.id = a.therapist_id
JOIN appointment_slots s ON s.tenant_id = a.tenant_id AND s.id = a.slot_id
WHERE a.tenant_id = $1 AND a.id = $2;

-- name: ListAdminAppointments :many
SELECT a.*, t.name as therapist_name, s.starts_at, s.ends_at
FROM appointments a
JOIN therapists t ON t.tenant_id = a.tenant_id AND t.id = a.therapist_id
JOIN appointment_slots s ON s.tenant_id = a.tenant_id AND s.id = a.slot_id
WHERE a.tenant_id = $1
  AND (sqlc.narg('therapist_id')::uuid IS NULL OR a.therapist_id = sqlc.narg('therapist_id'))
  AND (sqlc.narg('status')::text IS NULL OR a.status = sqlc.narg('status'))
ORDER BY a.created_at DESC;

-- name: UpdateAppointmentSlot :one
UPDATE appointments
SET slot_id = $3,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;


