-- name: CreateTaskEntry :exec
INSERT INTO tasks ( type, status, submitted_at, started_at, completed_at, result_path, details)
VALUES ( $1, $2, $3, $4, $5, $6, $7);

-- name: GetTaskEntry :one
SELECT id, type,  status, submitted_at, started_at, completed_at, result_path, details
FROM tasks
WHERE id = $1;
