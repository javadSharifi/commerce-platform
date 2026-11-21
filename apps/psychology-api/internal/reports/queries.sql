-- name: GetWorkshopMetrics :one
SELECT
    COUNT(*)::bigint AS total_workshops,
    COUNT(*) FILTER (WHERE status = 'published')::bigint AS published_workshops,
    COUNT(*) FILTER (WHERE status = 'draft')::bigint AS draft_workshops,
    COUNT(*) FILTER (WHERE status = 'closed')::bigint AS closed_workshops,
    COUNT(*) FILTER (WHERE status = 'cancelled')::bigint AS cancelled_workshops
FROM workshops
WHERE tenant_id = $1;

-- name: GetRegistrationMetrics :one
SELECT
    COUNT(*)::bigint AS total_registrations,
    COUNT(*) FILTER (WHERE status = 'confirmed')::bigint AS confirmed_registrations,
    COUNT(*) FILTER (WHERE status = 'pending')::bigint AS pending_registrations,
    COUNT(*) FILTER (WHERE status = 'cancelled')::bigint AS cancelled_registrations
FROM workshop_registrations
WHERE tenant_id = $1;

-- name: GetTherapistCount :one
SELECT COUNT(*)::bigint AS active_therapists
FROM therapists
WHERE tenant_id = $1 AND is_active = true AND deleted_at IS NULL;

-- name: GetSlotMetrics :one
SELECT
    COUNT(*)::bigint AS total_slots,
    COUNT(*) FILTER (WHERE status = 'available')::bigint AS available_slots,
    COUNT(*) FILTER (WHERE status = 'booked')::bigint AS booked_slots,
    COUNT(*) FILTER (WHERE status = 'held')::bigint AS held_slots
FROM appointment_slots
WHERE tenant_id = $1;

-- name: GetAppointmentMetrics :one
SELECT
    COUNT(*)::bigint AS total_appointments,
    COUNT(*) FILTER (WHERE status = 'confirmed')::bigint AS confirmed_appointments,
    COUNT(*) FILTER (WHERE status = 'pending')::bigint AS pending_appointments,
    COUNT(*) FILTER (WHERE status = 'completed')::bigint AS completed_appointments,
    COUNT(*) FILTER (WHERE status = 'cancelled')::bigint AS cancelled_appointments
FROM appointments
WHERE tenant_id = $1;

-- name: GetCourseMetrics :one
SELECT
    COUNT(*)::bigint AS total_courses,
    COUNT(*) FILTER (WHERE status = 'published')::bigint AS published_courses,
    COUNT(*) FILTER (WHERE status = 'draft')::bigint AS draft_courses,
    COUNT(*) FILTER (WHERE status = 'archived')::bigint AS archived_courses
FROM courses
WHERE tenant_id = $1;

-- name: GetLessonCount :one
SELECT COUNT(*)::bigint AS total_lessons
FROM course_lessons
WHERE tenant_id = $1;

-- name: GetEntitlementMetrics :one
SELECT
    COUNT(*)::bigint AS total_entitlements,
    COUNT(*) FILTER (WHERE status = 'active')::bigint AS active_entitlements,
    COUNT(*) FILTER (WHERE status = 'revoked')::bigint AS revoked_entitlements,
    COUNT(*) FILTER (WHERE status = 'pending')::bigint AS pending_entitlements
FROM course_entitlements
WHERE tenant_id = $1;

-- name: GetFinancialMetrics :one
SELECT
    COUNT(*)::bigint AS total_purchases,
    COUNT(*) FILTER (WHERE status = 'completed')::bigint AS completed_purchases,
    COUNT(*) FILTER (WHERE status = 'pending')::bigint AS pending_purchases,
    COUNT(*) FILTER (WHERE status = 'failed')::bigint AS failed_purchases,
    COUNT(*) FILTER (WHERE status = 'refund_required')::bigint AS refund_required_purchases,
    COUNT(*) FILTER (WHERE status = 'refunded')::bigint AS refunded_purchases,
    COALESCE(SUM(amount_rials) FILTER (WHERE status IN ('completed', 'refunded', 'refund_required')), 0)::bigint AS gross_revenue_rials,
    COALESCE(SUM(amount_rials) FILTER (WHERE status = 'completed' AND item_type = 'workshop'), 0)::bigint AS workshop_revenue_rials,
    COALESCE(SUM(amount_rials) FILTER (WHERE status = 'completed' AND item_type = 'course'), 0)::bigint AS course_revenue_rials
FROM purchases
WHERE tenant_id = $1;

-- name: GetRefundMetrics :one
SELECT
    COUNT(*)::bigint AS total_refunds,
    COALESCE(SUM(amount_rials), 0)::bigint AS total_refunded_rials
FROM refunds
WHERE tenant_id = $1;

-- name: GetPaymentAttemptMetrics :one
SELECT
    COUNT(*)::bigint AS total_attempts,
    COUNT(*) FILTER (WHERE status = 'initiated')::bigint AS initiated_attempts,
    COUNT(*) FILTER (WHERE status = 'verified')::bigint AS verified_attempts,
    COUNT(*) FILTER (WHERE status = 'failed')::bigint AS failed_attempts
FROM payment_attempts
WHERE tenant_id = $1;

-- name: GetMediaMetrics :one
SELECT
    COUNT(*)::bigint AS total_assets,
    COALESCE(SUM(size_bytes), 0)::bigint AS total_bytes,
    COALESCE(SUM(size_bytes) FILTER (WHERE access_class = 'public'), 0)::bigint AS public_bytes,
    COALESCE(SUM(size_bytes) FILTER (WHERE access_class = 'course_private'), 0)::bigint AS course_private_bytes,
    COUNT(*) FILTER (WHERE mime_type LIKE 'video/%')::bigint AS video_count,
    COUNT(*) FILTER (WHERE mime_type LIKE 'audio/%')::bigint AS audio_count,
    COUNT(*) FILTER (WHERE mime_type LIKE 'image/%')::bigint AS image_count
FROM media_assets
WHERE tenant_id = $1 AND validation_status = 'verified';
