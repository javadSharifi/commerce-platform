-- name: CreateBlogPost :one
INSERT INTO blog_posts (
    tenant_id,
    title,
    slug,
    seo_description,
    content_blocks,
    status,
    published_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetBlogPostByID :one
SELECT * FROM blog_posts
WHERE tenant_id = $1 AND id = $2;

-- name: GetPublishedBlogPostBySlug :one
SELECT * FROM blog_posts
WHERE tenant_id = $1 AND slug = $2 AND status = 'published';

-- name: ListPublishedBlogPosts :many
SELECT id, tenant_id, title, slug, seo_description, published_at
FROM blog_posts
WHERE tenant_id = $1 AND status = 'published'
ORDER BY published_at DESC;

-- name: ListAdminBlogPosts :many
SELECT * FROM blog_posts
WHERE tenant_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY updated_at DESC;

-- name: UpdateBlogPost :one
UPDATE blog_posts
SET title = $3,
    slug = $4,
    seo_description = $5,
    content_blocks = $6,
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
