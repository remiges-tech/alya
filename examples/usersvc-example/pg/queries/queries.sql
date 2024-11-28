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