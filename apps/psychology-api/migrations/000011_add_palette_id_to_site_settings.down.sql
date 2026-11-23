-- 000011_add_palette_id_to_site_settings.down.sql

ALTER TABLE site_settings
    DROP COLUMN IF EXISTS palette_id;
