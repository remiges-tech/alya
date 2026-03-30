package app

import (
	"context"
	"errors"

	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/api"
	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/repository"
)

var (
	ErrOrderNotFound   = errors.New("order not found")
	ErrOrderNumberUsed = errors.New("order number already exists")
)

type OrderService struct {
	orders repository.OrderRepository
	users  repository.UserRepository
}

func NewOrderService(orders repository.OrderRepository, users repository.UserRepository) *OrderService {
	return &OrderService{orders: orders, users: users}
}

func (s *OrderService) CreateOrder(ctx context.Context, req api.CreateOrderRequest) (repository.Order, error) {
	if _, err := s.users.GetUser(ctx, req.UserID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.Order{}, ErrUserNotFound
		}
		return repository.Order{}, err
	}
	if _, exists, err := s.orders.FindByNumber(ctx, req.Number); err != nil {
		return repository.Order{}, err
	} else if exists {
		return repository.Order{}, ErrOrderNumberUsed
	}

	return s.orders.CreateOrder(ctx, repository.Order{
		UserID:      req.UserID,
		Number:      req.Number,
		Status:      req.Status,
		TotalAmount: req.TotalAmount,
	})
}

func (s *OrderService) GetOrder(ctx context.Context, id int64) (repository.Order, error) {
	order, err := s.orders.GetOrder(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.Order{}, ErrOrderNotFound
		}
		return repository.Order{}, err
	}
	return order, nil
}

func (s *OrderService) ListOrders(ctx context.Context) ([]repository.Order, error) {
	return s.orders.ListOrders(ctx)
}

func (s *OrderService) UpdateOrder(ctx context.Context, id int64, req api.UpdateOrderRequest) (repository.Order, error) {
	order, err := s.orders.GetOrder(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.Order{}, ErrOrderNotFound
		}
		return repository.Order{}, err
	}

	if userID, ok := req.UserID.Get(); ok {
		if _, err := s.users.GetUser(ctx, userID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return repository.Order{}, ErrUserNotFound
			}
			return repository.Order{}, err
		}
		order.UserID = userID
	}
	if number, ok := req.Number.Get(); ok {
		if number != order.Number {
			if _, exists, err := s.orders.FindByNumber(ctx, number); err != nil {
				return repository.Order{}, err
			} else if exists {
				return repository.Order{}, ErrOrderNumberUsed
			}
		}
		order.Number = number
	}
	if status, ok := req.Status.Get(); ok {
		order.Status = status
	}
	if totalAmount, ok := req.TotalAmount.Get(); ok {
		order.TotalAmount = totalAmount
	}

	updatedOrder, err := s.orders.UpdateOrder(ctx, order)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.Order{}, ErrOrderNotFound
		}
		return repository.Order{}, err
	}
	return updatedOrder, nil
}

func (s *OrderService) DeleteOrder(ctx context.Context, id int64) error {
	if err := s.orders.DeleteOrder(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrOrderNotFound
		}
		return err
	}
	return nil
}
