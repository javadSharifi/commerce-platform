-- 000008_add_site_template_settings.down.sql

ALTER TABLE site_settings
    DROP COLUMN IF EXISTS theme_scale,
    DROP COLUMN IF EXISTS theme_radius,
    DROP COLUMN IF EXISTS template_version,
    DROP COLUMN IF EXISTS template_id;
