-- Constraint validation blocks rollback while a site still uses Stitch.
ALTER TABLE site_settings
    DROP CONSTRAINT site_settings_palette_id_check,
    ADD CONSTRAINT site_settings_palette_id_check
        CHECK (palette_id IN ('calm-sage', 'ocean-trust', 'warm-sand', 'royal-plum'));
