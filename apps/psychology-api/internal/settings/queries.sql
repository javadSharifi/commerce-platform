-- name: GetSiteSettings :one
SELECT tenant_id, title, description, phone, address, primary_color, logo_media_id, created_at, updated_at
FROM site_settings
WHERE tenant_id = $1;

-- name: UpsertSiteSettings :one
INSERT INTO site_settings (tenant_id, title, description, phone, address, primary_color, logo_media_id, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (tenant_id) DO UPDATE
SET title = EXCLUDED.title,
    description = EXCLUDED.description,
    phone = EXCLUDED.phone,
    address = EXCLUDED.address,
    primary_color = EXCLUDED.primary_color,
    logo_media_id = EXCLUDED.logo_media_id,
    updated_at = now()
RETURNING tenant_id, title, description, phone, address, primary_color, logo_media_id, created_at, updated_at;

-- name: GetPublicSiteProfile :one
SELECT t.name as clinic_name,
       s.title, s.description, s.phone, s.address, s.primary_color, s.logo_media_id
FROM tenants t
LEFT JOIN site_settings s ON s.tenant_id = t.id
WHERE t.id = $1 AND t.status = 'active';
