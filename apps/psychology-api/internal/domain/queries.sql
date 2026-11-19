-- name: GetDomainByHost :one
SELECT id, tenant_id, host, is_primary, is_verified, verified_at, created_at, updated_at
FROM domains
WHERE host = $1;

-- name: ListDomainsByTenant :many
SELECT id, tenant_id, host, is_primary, is_verified, verified_at, created_at, updated_at
FROM domains
WHERE tenant_id = $1
ORDER BY is_primary DESC, created_at ASC;
