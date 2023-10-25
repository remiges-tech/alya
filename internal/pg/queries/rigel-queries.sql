-- name: CreateSchema :one
INSERT INTO schema (name, description, tags, fields, created_by, updated_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
RETURNING *;

-- name: CreateConfig :one
INSERT INTO config (name, description, active, tags, schema_id, values, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: CheckSchemaExists :one
SELECT EXISTS(SELECT 1 FROM schema WHERE id=$1);
