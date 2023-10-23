-- name: CreateSchema :one
INSERT INTO schema (name, description, tags, active, active_version_id, created_by, updated_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
RETURNING *;
