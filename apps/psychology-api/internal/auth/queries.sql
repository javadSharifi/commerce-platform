-- name: GetPanelIdentityByEmail :one
SELECT id, email, password_hash, full_name, status, created_at, updated_at
FROM panel_identities
WHERE email = $1;

-- name: GetPanelIdentityByID :one
SELECT id, email, password_hash, full_name, status, created_at, updated_at
FROM panel_identities
WHERE id = $1;

-- name: CreatePanelIdentity :one
INSERT INTO panel_identities (id, email, password_hash, full_name, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, password_hash, full_name, status, created_at, updated_at;

-- name: CreatePanelSession :one
INSERT INTO panel_sessions (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, token_hash, expires_at, created_at;

-- name: GetPanelSessionByTokenHash :one
SELECT id, user_id, token_hash, expires_at, created_at
FROM panel_sessions
WHERE token_hash = $1 AND expires_at > now();

-- name: DeletePanelSession :exec
DELETE FROM panel_sessions
WHERE token_hash = $1;

-- name: ListMembershipsByUserID :many
SELECT m.id, m.tenant_id, m.user_id, m.role, m.created_at, m.updated_at,
       t.name as tenant_name, t.slug as tenant_slug
FROM memberships m
JOIN tenants t ON t.id = m.tenant_id
WHERE m.user_id = $1;

-- name: GetMembershipByUserAndTenant :one
SELECT id, tenant_id, user_id, role, created_at, updated_at
FROM memberships
WHERE user_id = $1 AND tenant_id = $2;

-- name: CreateMembership :one
INSERT INTO memberships (id, tenant_id, user_id, role)
VALUES ($1, $2, $3, $4)
RETURNING id, tenant_id, user_id, role, created_at, updated_at;
