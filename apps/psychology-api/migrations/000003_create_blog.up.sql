-- 000003_create_blog.up.sql
-- Blog posts with Tenant-scoped RLS, composite unique constraints, and structured content blocks

CREATE TABLE blog_posts (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title           text        NOT NULL,
    slug            text        NOT NULL,
    seo_description text        NOT NULL DEFAULT '',
    content_blocks  jsonb       NOT NULL DEFAULT '[]'::jsonb,
    status          text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    published_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT blog_posts_tenant_id_id_uq UNIQUE (tenant_id, id),
    CONSTRAINT blog_posts_tenant_slug_uq UNIQUE (tenant_id, slug)
);

CREATE INDEX blog_posts_tenant_status_published_idx ON blog_posts (tenant_id, status, published_at DESC);

ALTER TABLE blog_posts ENABLE ROW LEVEL SECURITY;
ALTER TABLE blog_posts FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON blog_posts
    USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON blog_posts TO app_user;
