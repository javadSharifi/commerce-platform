ALTER TABLE blog_posts
    ADD COLUMN author_name text NOT NULL DEFAULT '' CHECK (char_length(author_name) <= 120),
    ADD COLUMN category text NOT NULL DEFAULT '' CHECK (char_length(category) <= 80),
    ADD COLUMN is_featured boolean NOT NULL DEFAULT false,
    ADD COLUMN cover_media_id uuid,
    ADD CONSTRAINT blog_posts_cover_media_fk
        FOREIGN KEY (tenant_id, cover_media_id)
        REFERENCES media_assets (tenant_id, id)
        ON DELETE SET NULL (cover_media_id);
