package sqlc

import "database/sql"

type Order struct {
	ID          int32        `json:"id"`
	UserID      int32        `json:"user_id"`
	Number      string       `json:"number"`
	Status      string       `json:"status"`
	TotalAmount int64        `json:"total_amount"`
	CreatedAt   sql.NullTime `json:"created_at"`
	UpdatedAt   sql.NullTime `json:"updated_at"`
}
