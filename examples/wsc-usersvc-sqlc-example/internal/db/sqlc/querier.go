package sqlc

import (
	"context"
)

// Querier is the SQLC-generated query interface.
//
// In this repository, the file is checked in directly so the example builds
// without requiring sqlc during the task. With emit_interface enabled in
// sqlc.yaml, sqlc will generate the same shape on regeneration.
type Querier interface {
	CheckUsernameExists(ctx context.Context, username string) (bool, error)
	CreateUser(ctx context.Context, arg CreateUserParams) (int32, error)
	GetUser(ctx context.Context, id int32) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	UpdateUser(ctx context.Context, arg UpdateUserParams) (User, error)
	DeleteUser(ctx context.Context, id int32) (int64, error)

	CreateOrder(ctx context.Context, arg CreateOrderParams) (int32, error)
	GetOrder(ctx context.Context, id int32) (Order, error)
	ListOrders(ctx context.Context) ([]Order, error)
	GetOrderByNumber(ctx context.Context, number string) (Order, error)
	UpdateOrder(ctx context.Context, arg UpdateOrderParams) (Order, error)
	DeleteOrder(ctx context.Context, id int32) (int64, error)
}

var _ Querier = (*Queries)(nil)
