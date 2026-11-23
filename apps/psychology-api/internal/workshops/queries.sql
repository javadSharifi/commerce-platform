-- name: CreateWorkshop :one
INSERT INTO workshops (
    tenant_id,
    title,
    slug,
    description,
    instructor_name,
    starts_at,
    ends_at,
    location,
    registration_deadline,
    capacity,
    price_rials,
    status,
    topic
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING *;

-- name: GetWorkshopByID :one
SELECT * FROM workshops
WHERE tenant_id = $1 AND id = $2;

-- name: GetWorkshopByIDForUpdate :one
SELECT * FROM workshops
WHERE tenant_id = $1 AND id = $2
FOR UPDATE;

-- name: GetWorkshopBySlug :one
SELECT * FROM workshops
WHERE tenant_id = $1 AND slug = $2;

-- name: ListWorkshops :many
SELECT * FROM workshops
WHERE tenant_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY starts_at DESC;

-- name: ListPublishedWorkshops :many
SELECT * FROM workshops
WHERE tenant_id = $1
  AND status = 'published'
  AND ends_at >= now()
ORDER BY starts_at ASC;

-- name: UpdateWorkshop :one
UPDATE workshops
SET title = $3,
    slug = $4,
    description = $5,
    instructor_name = $6,
    starts_at = $7,
    ends_at = $8,
    location = $9,
    registration_deadline = $10,
    capacity = $11,
    price_rials = $12,
    topic = COALESCE(sqlc.narg('topic')::text, topic),
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- name: UpdateWorkshopStatus :one
UPDATE workshops
SET status = $3,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- name: UpdateWorkshopCapacity :one
UPDATE workshops
SET capacity = $3,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- name: CountActiveRegistrations :one
SELECT COUNT(*)::bigint
FROM workshop_registrations
WHERE tenant_id = $1 AND workshop_id = $2
  AND (status = 'confirmed' OR (status = 'pending_payment' AND expires_at > now()));

-- name: CountConfirmedRegistrations :one
SELECT COUNT(*)::bigint
FROM workshop_registrations
WHERE tenant_id = $1 AND workshop_id = $2
  AND status = 'confirmed';

-- name: CreateRegistration :one
INSERT INTO workshop_registrations (
    tenant_id,
    workshop_id,
    guest_name,
    guest_phone,
    status,
    expires_at,
    purchase_id,
    guest_lookup_hash
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetRegistrationByID :one
SELECT * FROM workshop_registrations
WHERE tenant_id = $1 AND id = $2;

-- name: GetRegistrationByIDForUpdate :one
SELECT * FROM workshop_registrations
WHERE tenant_id = $1 AND id = $2
FOR UPDATE;

-- name: GetRegistrationByPurchaseID :one
SELECT * FROM workshop_registrations
WHERE tenant_id = $1 AND purchase_id = $2;

-- name: GetRegistrationByPurchaseIDForUpdate :one
SELECT * FROM workshop_registrations
WHERE tenant_id = $1 AND purchase_id = $2
FOR UPDATE;

-- name: GetRegistrationByLookupHash :one
SELECT * FROM workshop_registrations
WHERE tenant_id = $1 AND guest_lookup_hash = $2;

-- name: GetActiveRegistrationByPhone :one
SELECT * FROM workshop_registrations
WHERE tenant_id = $1 AND workshop_id = $2 AND guest_phone = $3
  AND (status = 'confirmed' OR (status = 'pending_payment' AND expires_at > now()))
LIMIT 1;

-- name: ListRegistrationsByWorkshop :many
SELECT * FROM workshop_registrations
WHERE tenant_id = $1 AND workshop_id = $2
ORDER BY created_at ASC;

-- name: UpdateRegistrationStatus :one
UPDATE workshop_registrations
SET status = $3,
    purchase_id = COALESCE($4, purchase_id),
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- name: ExpirePendingRegistrations :execrows
UPDATE workshop_registrations
SET status = 'expired',
    updated_at = now()
WHERE status = 'pending_payment'
  AND expires_at <= now();
