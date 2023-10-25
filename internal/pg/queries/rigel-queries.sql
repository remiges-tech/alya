-- name: CreateSchema :one
INSERT INTO schema (name, description, tags, active, active_version_id, created_by, updated_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
RETURNING *;

-- name: CreateConfig :one
INSERT INTO config (name, description, active, tags, schema_version_id, values, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: CheckSchemaExists :one
SELECT EXISTS(SELECT 1 FROM schema WHERE id=$1);

-- name: CreateSchemaVersion :one
INSERT INTO schema_versions (schema_id, version, fields, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;