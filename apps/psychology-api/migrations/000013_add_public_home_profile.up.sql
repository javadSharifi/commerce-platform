ALTER TABLE site_settings
    ADD COLUMN home_profile jsonb NOT NULL DEFAULT '{}'::jsonb
        CHECK (jsonb_typeof(home_profile) = 'object'),
    ADD COLUMN portrait_media_id uuid,
    ADD CONSTRAINT site_settings_portrait_fk
        FOREIGN KEY (tenant_id, portrait_media_id)
        REFERENCES media_assets (tenant_id, id)
        ON DELETE SET NULL (portrait_media_id);
