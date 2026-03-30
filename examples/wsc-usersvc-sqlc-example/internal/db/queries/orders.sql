-- name: CreateOrder :one
INSERT INTO orders (
    user_id,
    number,
    status,
    total_amount,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING id;

-- name: GetOrder :one
SELECT id, user_id, number, status, total_amount, created_at, updated_at
FROM orders
WHERE id = $1;

-- name: ListOrders :many
SELECT id, user_id, number, status, total_amount, created_at, updated_at
FROM orders
ORDER BY id;

-- name: GetOrderByNumber :one
SELECT id, user_id, number, status, total_amount, created_at, updated_at
FROM orders
WHERE number = $1;

-- name: UpdateOrder :one
UPDATE orders
SET user_id = $2,
    number = $3,
    status = $4,
    total_amount = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, user_id, number, status, total_amount, created_at, updated_at;

-- name: DeleteOrder :execrows
DELETE FROM orders
WHERE id = $1;
