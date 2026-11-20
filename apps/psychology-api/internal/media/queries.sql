-- apps/psychology-api/internal/media/queries.sql

-- name: CreateMediaUpload :one
INSERT INTO media_uploads (
    id,
    tenant_id,
    staging_key,
    original_name,
    access_class,
    expected_mime_type,
    expected_size_bytes,
    expires_at,
    status,
    initiated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, 'reserved', $9
)
RETURNING *;

-- name: GetMediaUploadByID :one
SELECT * FROM media_uploads
WHERE tenant_id = $1 AND id = $2;

-- name: GetMediaUploadByStagingKey :one
SELECT * FROM media_uploads
WHERE tenant_id = $1 AND staging_key = $2;

-- name: MarkMediaUploadCompleted :one
UPDATE media_uploads
SET status = 'completed'
WHERE tenant_id = $1 AND id = $2 AND status = 'reserved'
RETURNING *;

-- name: MarkMediaUploadExpired :exec
UPDATE media_uploads
SET status = 'expired'
WHERE tenant_id = $1 AND id = $2;

-- name: ListExpiredMediaUploads :many
SELECT * FROM media_uploads
WHERE status = 'reserved' AND expires_at < $1
ORDER BY expires_at ASC
LIMIT $2;

-- name: GetTotalMediaUsageBytes :one
SELECT 
    (COALESCE((SELECT SUM(ma.size_bytes) FROM media_assets ma WHERE ma.tenant_id = $1 AND ma.validation_status = 'verified'), 0) +
     COALESCE((SELECT SUM(mu.expected_size_bytes) FROM media_uploads mu WHERE mu.tenant_id = $1 AND mu.status = 'reserved' AND mu.expires_at > $2), 0))::bigint AS total_used_bytes;

-- name: CreateMediaAsset :one
INSERT INTO media_assets (
    tenant_id,
    storage_key,
    original_name,
    mime_type,
    size_bytes,
    access_class,
    validation_status,
    uploaded_by,
    verified_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetMediaAssetByID :one
SELECT * FROM media_assets
WHERE tenant_id = $1 AND id = $2;

-- name: GetMediaAssetByStorageKey :one
SELECT * FROM media_assets
WHERE tenant_id = $1 AND storage_key = $2;

-- name: ListMediaAssetsByTenant :many
SELECT * FROM media_assets
WHERE tenant_id = $1
ORDER BY created_at DESC;
