-- 000008_add_site_template_settings.up.sql
-- Add template configuration to site_settings

ALTER TABLE site_settings
    ADD COLUMN template_id      text NOT NULL DEFAULT 'minimal',
    ADD COLUMN template_version text NOT NULL DEFAULT '1.0.0',
    ADD COLUMN theme_radius     text NOT NULL DEFAULT 'md' CHECK (theme_radius IN ('sm', 'md', 'lg')),
    ADD COLUMN theme_scale      text NOT NULL DEFAULT 'md' CHECK (theme_scale IN ('sm', 'md', 'lg'));
