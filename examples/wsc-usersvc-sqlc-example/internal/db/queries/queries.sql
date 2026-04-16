-- name: CreateUser :one
INSERT INTO users (
    name,
    email,
    username,
    created_at,
    updated_at,
    phone_number
) VALUES (
    $1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $4
) RETURNING id;

-- name: CheckUsernameExists :one
SELECT EXISTS(
    SELECT 1 FROM users WHERE username = $1
) AS exists;

-- name: GetUser :one
SELECT id, name, email, username, phone_number, created_at, updated_at
FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT id, name, email, username, phone_number, created_at, updated_at
FROM users
ORDER BY id;

-- name: GetUserByUsername :one
SELECT id, name, email, username, phone_number, created_at, updated_at
FROM users
WHERE username = $1;

-- name: UpdateUser :one
UPDATE users
SET name = $2,
    email = $3,
    username = $4,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, name, email, username, phone_number, created_at, updated_at;

-- name: DeleteUser :execrows
DELETE FROM users
WHERE id = $1;