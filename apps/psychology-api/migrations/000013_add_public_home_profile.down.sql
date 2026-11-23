ALTER TABLE site_settings
    DROP CONSTRAINT site_settings_portrait_fk,
    DROP COLUMN portrait_media_id,
    DROP COLUMN home_profile;
