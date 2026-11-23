-- name: CreateBlogPost :one
INSERT INTO blog_posts (
    tenant_id,
    title,
    slug,
    seo_description,
    content_blocks,
    status,
    published_at,
    author_name,
    category,
    is_featured,
    cover_media_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: GetBlogPostByID :one
SELECT * FROM blog_posts
WHERE tenant_id = $1 AND id = $2;

-- name: LockBlogPost :one
SELECT * FROM blog_posts
WHERE tenant_id = $1 AND id = $2
FOR UPDATE;

-- name: GetPublishedBlogPostBySlug :one
SELECT * FROM blog_posts
WHERE tenant_id = $1 AND slug = $2 AND status = 'published';

-- name: ListPublishedBlogPosts :many
SELECT id, tenant_id, title, slug, seo_description, published_at, author_name, category, is_featured, cover_media_id
FROM blog_posts
WHERE tenant_id = $1 AND status = 'published'
ORDER BY published_at DESC NULLS LAST, id DESC;

-- name: ListPublishedSlugsByCategories :many
SELECT slug FROM blog_posts
WHERE tenant_id = $1 AND status = 'published' AND category <> ''
  AND category = ANY(sqlc.arg('categories')::text[])
ORDER BY slug;

-- name: ListAdminBlogPosts :many
SELECT * FROM blog_posts
WHERE tenant_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY updated_at DESC;

-- name: UpdateBlogPost :one
UPDATE blog_posts
SET title = $3,
    slug = $4,
    seo_description = COALESCE(sqlc.narg('seo_description')::text, seo_description),
    content_blocks = COALESCE(sqlc.narg('content_blocks')::jsonb, content_blocks),
    author_name = COALESCE(sqlc.narg('author_name')::text, author_name),
    category = COALESCE(sqlc.narg('category')::text, category),
    is_featured = COALESCE(sqlc.narg('is_featured')::boolean, is_featured),
    cover_media_id = CASE WHEN sqlc.arg('replace_cover')::boolean
        THEN sqlc.narg('cover_media_id')::uuid ELSE cover_media_id END,
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- name: PublishBlogPost :one
UPDATE blog_posts
SET status = 'published',
    published_at = COALESCE(published_at, now()),
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- name: UnpublishBlogPost :one
UPDATE blog_posts
SET status = 'draft',
    updated_at = now()
WHERE tenant_id = $1 AND id = $2
RETURNING *;

-- name: DeleteBlogPost :exec
DELETE FROM blog_posts
WHERE tenant_id = $1 AND id = $2;
