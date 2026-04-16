package repository

import "context"

type Order struct {
	ID          int64
	UserID      int64
	Number      string
	Status      string
	TotalAmount int64
}

type OrderRepository interface {
	CreateOrder(ctx context.Context, order Order) (Order, error)
	GetOrder(ctx context.Context, id int64) (Order, error)
	ListOrders(ctx context.Context) ([]Order, error)
	FindByNumber(ctx context.Context, number string) (Order, bool, error)
	UpdateOrder(ctx context.Context, order Order) (Order, error)
	DeleteOrder(ctx context.Context, id int64) error
}
