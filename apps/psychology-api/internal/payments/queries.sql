-- name: CreatePurchase :one
INSERT INTO purchases (
    tenant_id,
    item_type,
    item_id,
    amount_rials,
    title_snapshot,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetPurchaseByID :one
SELECT * FROM purchases
WHERE tenant_id = $1 AND id = $2;

-- name: GetPurchaseByIDForUpdate :one
SELECT * FROM purchases
WHERE tenant_id = $1 AND id = $2
FOR UPDATE;

-- name: UpdatePurchaseStatus :one
UPDATE purchases
SET status = $3,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- name: ListPurchases :many
SELECT * FROM purchases
WHERE tenant_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC;

-- name: CreatePaymentAttempt :one
INSERT INTO payment_attempts (
    tenant_id,
    purchase_id,
    provider,
    authority,
    amount_rials,
    status,
    idempotency_key
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetPaymentAttemptByAuthority :one
SELECT * FROM payment_attempts
WHERE tenant_id = $1 AND authority = $2;

-- name: GetPaymentAttemptByIdempotencyKey :one
SELECT * FROM payment_attempts
WHERE tenant_id = $1 AND idempotency_key = $2;

-- name: UpdatePaymentAttemptStatus :one
UPDATE payment_attempts
SET status = $3,
    tracking_code = COALESCE($4, tracking_code),
    gateway_ref_id = COALESCE($5, gateway_ref_id),
    error_message = COALESCE($6, error_message),
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- name: ListPaymentAttemptsByPurchase :many
SELECT * FROM payment_attempts
WHERE tenant_id = $1 AND purchase_id = $2
ORDER BY created_at DESC;

-- name: GetActivePaymentAttemptByPurchase :one
SELECT * FROM payment_attempts
WHERE tenant_id = $1 AND purchase_id = $2 AND status = 'initiated'
ORDER BY created_at DESC
LIMIT 1;

-- name: GetVerifiedPaymentAttemptByPurchase :one
SELECT * FROM payment_attempts
WHERE tenant_id = $1 AND purchase_id = $2 AND status = 'verified'
ORDER BY created_at DESC
LIMIT 1;

-- name: ListStalePaymentAttempts :many
SELECT * FROM payment_attempts
WHERE tenant_id = $1
  AND status = 'initiated'
  AND created_at <= $2
ORDER BY created_at ASC
LIMIT $3;

-- name: CreateRefund :one
INSERT INTO refunds (
    tenant_id,
    purchase_id,
    payment_attempt_id,
    amount_rials,
    reason,
    reference_number,
    registered_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetRefundByReference :one
SELECT * FROM refunds
WHERE tenant_id = $1 AND reference_number = $2;

-- name: ListRefundsByPurchase :many
SELECT * FROM refunds
WHERE tenant_id = $1 AND purchase_id = $2
ORDER BY registered_at DESC;
