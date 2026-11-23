-- 000011_add_palette_id_to_site_settings.up.sql
-- Add palette_id to site_settings with default valid palette 'calm-sage'

ALTER TABLE site_settings
    ADD COLUMN palette_id text NOT NULL DEFAULT 'calm-sage' CHECK (palette_id IN ('calm-sage', 'ocean-trust', 'warm-sand', 'royal-plum'));
