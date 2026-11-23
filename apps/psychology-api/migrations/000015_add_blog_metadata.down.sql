ALTER TABLE blog_posts
    DROP CONSTRAINT blog_posts_cover_media_fk,
    DROP COLUMN cover_media_id,
    DROP COLUMN is_featured,
    DROP COLUMN category,
    DROP COLUMN author_name;
