package sqlc

import "context"

const createOrder = `-- name: CreateOrder :one
INSERT INTO orders (
    user_id,
    number,
    status,
    total_amount,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING id
`

type CreateOrderParams struct {
	UserID      int32  `json:"user_id"`
	Number      string `json:"number"`
	Status      string `json:"status"`
	TotalAmount int64  `json:"total_amount"`
}

func (q *Queries) CreateOrder(ctx context.Context, arg CreateOrderParams) (int32, error) {
	row := q.db.QueryRowContext(ctx, createOrder, arg.UserID, arg.Number, arg.Status, arg.TotalAmount)
	var id int32
	err := row.Scan(&id)
	return id, err
}

const getOrder = `-- name: GetOrder :one
SELECT id, user_id, number, status, total_amount, created_at, updated_at
FROM orders
WHERE id = $1
`

func (q *Queries) GetOrder(ctx context.Context, id int32) (Order, error) {
	row := q.db.QueryRowContext(ctx, getOrder, id)
	var i Order
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.Number,
		&i.Status,
		&i.TotalAmount,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const listOrders = `-- name: ListOrders :many
SELECT id, user_id, number, status, total_amount, created_at, updated_at
FROM orders
ORDER BY id
`

func (q *Queries) ListOrders(ctx context.Context) ([]Order, error) {
	rows, err := q.db.QueryContext(ctx, listOrders)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Order{}
	for rows.Next() {
		var i Order
		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.Number,
			&i.Status,
			&i.TotalAmount,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getOrderByNumber = `-- name: GetOrderByNumber :one
SELECT id, user_id, number, status, total_amount, created_at, updated_at
FROM orders
WHERE number = $1
`

func (q *Queries) GetOrderByNumber(ctx context.Context, number string) (Order, error) {
	row := q.db.QueryRowContext(ctx, getOrderByNumber, number)
	var i Order
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.Number,
		&i.Status,
		&i.TotalAmount,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const updateOrder = `-- name: UpdateOrder :one
UPDATE orders
SET user_id = $2,
    number = $3,
    status = $4,
    total_amount = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, user_id, number, status, total_amount, created_at, updated_at
`

type UpdateOrderParams struct {
	ID          int32  `json:"id"`
	UserID      int32  `json:"user_id"`
	Number      string `json:"number"`
	Status      string `json:"status"`
	TotalAmount int64  `json:"total_amount"`
}

func (q *Queries) UpdateOrder(ctx context.Context, arg UpdateOrderParams) (Order, error) {
	row := q.db.QueryRowContext(ctx, updateOrder, arg.ID, arg.UserID, arg.Number, arg.Status, arg.TotalAmount)
	var i Order
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.Number,
		&i.Status,
		&i.TotalAmount,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const deleteOrder = `-- name: DeleteOrder :execrows
DELETE FROM orders
WHERE id = $1
`

func (q *Queries) DeleteOrder(ctx context.Context, id int32) (int64, error) {
	result, err := q.db.ExecContext(ctx, deleteOrder, id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
