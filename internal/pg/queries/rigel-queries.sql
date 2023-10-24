-- name: CreateSchema :one
INSERT INTO schema (name, description, tags, active, active_version_id, created_by, updated_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
RETURNING *;

-- name: CreateConfig :one
INSERT INTO config (name, description, active, tags, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;