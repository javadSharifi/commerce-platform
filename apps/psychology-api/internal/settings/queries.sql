-- name: GetSiteSettings :one
SELECT tenant_id, title, description, phone, address, primary_color, logo_media_id,
       template_id, template_version, theme_radius, theme_scale, palette_id, created_at, updated_at,
       home_profile::text AS home_profile, portrait_media_id
FROM site_settings
WHERE tenant_id = $1;

-- name: UpsertSiteSettings :one
INSERT INTO site_settings (
    tenant_id, title, description, phone, address, primary_color, logo_media_id,
    template_id, template_version, theme_radius, theme_scale, palette_id, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
ON CONFLICT (tenant_id) DO UPDATE
SET title = EXCLUDED.title,
    description = EXCLUDED.description,
    phone = EXCLUDED.phone,
    address = EXCLUDED.address,
    primary_color = EXCLUDED.primary_color,
    logo_media_id = EXCLUDED.logo_media_id,
    template_id = EXCLUDED.template_id,
    template_version = EXCLUDED.template_version,
    theme_radius = EXCLUDED.theme_radius,
    theme_scale = EXCLUDED.theme_scale,
    palette_id = EXCLUDED.palette_id,
    updated_at = now()
RETURNING tenant_id, title, description, phone, address, primary_color, logo_media_id,
          template_id, template_version, theme_radius, theme_scale, palette_id, created_at, updated_at;

-- name: UpsertSiteProfile :one
INSERT INTO site_settings (
    tenant_id, title, description, phone, address, primary_color, logo_media_id, updated_at,
    home_profile, portrait_media_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, now(),
        COALESCE(sqlc.narg(home_profile)::text::jsonb, '{}'::jsonb), sqlc.narg(portrait_media_id))
ON CONFLICT (tenant_id) DO UPDATE
SET title = EXCLUDED.title,
    description = EXCLUDED.description,
    phone = EXCLUDED.phone,
    address = EXCLUDED.address,
    primary_color = EXCLUDED.primary_color,
    logo_media_id = EXCLUDED.logo_media_id,
    home_profile = COALESCE(sqlc.narg(home_profile)::text::jsonb, site_settings.home_profile),
    portrait_media_id = CASE WHEN sqlc.narg(home_profile)::text IS NOT NULL
        THEN EXCLUDED.portrait_media_id ELSE site_settings.portrait_media_id END,
    updated_at = now()
RETURNING tenant_id, title, description, phone, address, primary_color, logo_media_id,
          template_id, template_version, theme_radius, theme_scale, palette_id, created_at, updated_at,
          home_profile::text AS home_profile, portrait_media_id;

-- name: GetSiteAppearance :one
SELECT tenant_id, template_id, template_version, palette_id, theme_radius, theme_scale, created_at, updated_at
FROM site_settings
WHERE tenant_id = $1;

-- name: UpsertSiteAppearance :one
INSERT INTO site_settings (
    tenant_id, template_id, template_version, palette_id, theme_radius, theme_scale, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (tenant_id) DO UPDATE
SET template_id = EXCLUDED.template_id,
    template_version = EXCLUDED.template_version,
    palette_id = EXCLUDED.palette_id,
    theme_radius = EXCLUDED.theme_radius,
    theme_scale = EXCLUDED.theme_scale,
    updated_at = now()
RETURNING tenant_id, template_id, template_version, palette_id, theme_radius, theme_scale, created_at, updated_at;

-- name: GetPublicSiteProfile :one
SELECT t.name as clinic_name,
       s.title, s.description, s.phone, s.address, s.primary_color, s.logo_media_id,
       COALESCE(s.template_id, 'minimal')::text as template_id,
       COALESCE(s.template_version, '1.0.0')::text as template_version,
       COALESCE(s.theme_radius, 'md')::text as theme_radius,
       COALESCE(s.theme_scale, 'md')::text as theme_scale,
       COALESCE(s.palette_id, 'calm-sage')::text as palette_id,
       COALESCE(s.home_profile, '{}'::jsonb)::text AS home_profile, s.portrait_media_id
FROM tenants t
LEFT JOIN site_settings s ON s.tenant_id = t.id
WHERE t.id = $1 AND t.status = 'active';

-- name: GetPrimaryVerifiedDomain :one
SELECT host
FROM domains
WHERE tenant_id = $1 AND is_verified = true
ORDER BY is_primary DESC, created_at ASC
LIMIT 1;
