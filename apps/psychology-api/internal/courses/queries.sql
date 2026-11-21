-- Buyer accounts queries
-- name: GetBuyerAccountByID :one
SELECT * FROM buyer_accounts
WHERE tenant_id = $1 AND id = $2;

-- name: GetBuyerAccountByPhone :one
SELECT * FROM buyer_accounts
WHERE tenant_id = $1 AND phone = $2;

-- name: CreateBuyerAccount :one
INSERT INTO buyer_accounts (
    tenant_id,
    phone,
    full_name,
    status
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: UpdateBuyerAccount :one
UPDATE buyer_accounts
SET full_name = $3,
    status = $4,
    updated_at = NOW()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- Buyer sessions queries
-- name: CreateBuyerSession :one
INSERT INTO buyer_sessions (
    tenant_id,
    buyer_id,
    token_hash,
    expires_at
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetBuyerSessionByTokenHash :one
SELECT s.id, s.tenant_id, s.buyer_id, s.token_hash, s.created_at, s.expires_at,
       b.phone, b.full_name, b.status AS buyer_status
FROM buyer_sessions s
JOIN buyer_accounts b ON b.tenant_id = s.tenant_id AND b.id = s.buyer_id
WHERE s.tenant_id = $1 AND s.token_hash = $2 AND s.expires_at > NOW();

-- name: DeleteBuyerSession :exec
DELETE FROM buyer_sessions
WHERE tenant_id = $1 AND token_hash = $2;

-- name: DeleteBuyerSessionsByBuyerID :exec
DELETE FROM buyer_sessions
WHERE tenant_id = $1 AND buyer_id = $2;

-- Buyer OTP codes queries
-- name: CreateBuyerOTPCode :one
INSERT INTO buyer_otp_codes (
    tenant_id,
    phone,
    code_hash,
    expires_at
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetLatestActiveBuyerOTP :one
SELECT * FROM buyer_otp_codes
WHERE tenant_id = $1 AND phone = $2 AND consumed_at IS NULL AND expires_at > $3
ORDER BY created_at DESC
LIMIT 1;

-- name: IncrementBuyerOTPAttempts :exec
UPDATE buyer_otp_codes
SET attempts = attempts + 1
WHERE id = $1;

-- name: ConsumeBuyerOTP :exec
UPDATE buyer_otp_codes
SET consumed_at = $2
WHERE id = $1;

-- name: InvalidateExistingBuyerOTPs :exec
UPDATE buyer_otp_codes
SET consumed_at = $3
WHERE tenant_id = $1 AND phone = $2 AND consumed_at IS NULL;

-- Courses queries
-- name: CreateCourse :one
INSERT INTO courses (
    tenant_id,
    title,
    slug,
    description,
    instructor_name,
    price_rials,
    cover_media_id,
    status,
    published_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetCourseByID :one
SELECT * FROM courses
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: GetCourseByIDForUpdate :one
SELECT * FROM courses
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
FOR UPDATE;

-- name: GetCourseBySlug :one
SELECT * FROM courses
WHERE tenant_id = $1 AND slug = $2 AND deleted_at IS NULL;

-- name: ListPublishedCourses :many
SELECT * FROM courses
WHERE tenant_id = $1 AND status = 'published' AND deleted_at IS NULL
ORDER BY published_at DESC;

-- name: ListAdminCourses :many
SELECT * FROM courses
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateCourse :one
UPDATE courses
SET title = $3,
    slug = $4,
    description = $5,
    instructor_name = $6,
    price_rials = $7,
    cover_media_id = $8,
    status = $9,
    published_at = $10,
    updated_at = NOW()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteCourse :exec
UPDATE courses
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL;

-- Course lessons queries
-- name: CreateCourseLesson :one
INSERT INTO course_lessons (
    tenant_id,
    course_id,
    title,
    description,
    display_order,
    media_asset_id,
    duration_seconds,
    is_free_preview
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: ListCourseLessons :many
SELECT * FROM course_lessons
WHERE tenant_id = $1 AND course_id = $2 AND deleted_at IS NULL
ORDER BY display_order ASC;

-- name: GetCourseLessonByID :one
SELECT * FROM course_lessons
WHERE tenant_id = $1 AND course_id = $2 AND id = $3 AND deleted_at IS NULL;

-- name: UpdateCourseLesson :one
UPDATE course_lessons
SET title = $4,
    description = $5,
    display_order = $6,
    media_asset_id = $7,
    duration_seconds = $8,
    is_free_preview = $9,
    updated_at = NOW()
WHERE tenant_id = $1 AND course_id = $2 AND id = $3 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteCourseLesson :exec
UPDATE course_lessons
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE tenant_id = $1 AND course_id = $2 AND id = $3 AND deleted_at IS NULL;

-- Course entitlements queries
-- name: CreateCourseEntitlement :one
INSERT INTO course_entitlements (
    tenant_id,
    buyer_id,
    course_id,
    purchase_id,
    status,
    course_title_snapshot,
    price_rials_snapshot,
    granted_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetActiveCourseEntitlement :one
SELECT * FROM course_entitlements
WHERE tenant_id = $1 AND buyer_id = $2 AND course_id = $3 AND status = 'active';

-- name: GetCourseEntitlementByBuyerAndCourse :one
SELECT * FROM course_entitlements
WHERE tenant_id = $1 AND buyer_id = $2 AND course_id = $3
ORDER BY created_at DESC
LIMIT 1;

-- name: ListBuyerCourseEntitlements :many
SELECT e.*, c.title AS current_course_title, c.slug AS course_slug, c.cover_media_id, c.status AS course_status
FROM course_entitlements e
JOIN courses c ON c.tenant_id = e.tenant_id AND c.id = e.course_id
WHERE e.tenant_id = $1 AND e.buyer_id = $2 AND e.status = 'active'
ORDER BY e.granted_at DESC;

-- name: ListCourseEntitlementsForAdmin :many
SELECT e.*, b.phone AS buyer_phone, b.full_name AS buyer_name
FROM course_entitlements e
JOIN buyer_accounts b ON b.tenant_id = e.tenant_id AND b.id = e.buyer_id
WHERE e.tenant_id = $1 AND e.course_id = $2
ORDER BY e.granted_at DESC;

-- name: RevokeCourseEntitlement :one
UPDATE course_entitlements
SET status = 'revoked',
    revoked_at = $4,
    revocation_reason = $5
WHERE tenant_id = $1 AND buyer_id = $2 AND course_id = $3 AND status = 'active'
RETURNING *;

-- name: GetCourseEntitlementByPurchaseID :one
SELECT * FROM course_entitlements
WHERE tenant_id = $1 AND purchase_id = $2;

-- name: ActivateCourseEntitlement :one
UPDATE course_entitlements
SET status = 'active',
    granted_at = $3,
    updated_at = NOW()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- name: RevokeCourseEntitlementByPurchaseID :one
UPDATE course_entitlements
SET status = 'revoked',
    revoked_at = $3,
    revocation_reason = $4,
    updated_at = NOW()
WHERE tenant_id = $1 AND purchase_id = $2 AND status = 'active'
RETURNING *;

