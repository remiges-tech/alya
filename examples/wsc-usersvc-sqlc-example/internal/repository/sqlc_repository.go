package repository

import (
	"context"
	"database/sql"

	pgsqlc "github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/db/sqlc"
)

type SQLCRepository struct {
	queries pgsqlc.Querier
}

func NewSQLCRepository(queries pgsqlc.Querier) *SQLCRepository {
	return &SQLCRepository{queries: queries}
}

func (r *SQLCRepository) CreateUser(ctx context.Context, user User) (User, error) {
	id, err := r.queries.CreateUser(ctx, pgsqlc.CreateUserParams{
		Name:        user.Name,
		Email:       user.Email,
		Username:    user.Username,
		PhoneNumber: sql.NullString{},
	})
	if err != nil {
		return User{}, err
	}
	created, err := r.queries.GetUser(ctx, id)
	if err != nil {
		return User{}, err
	}
	return toRepositoryUser(created), nil
}

func (r *SQLCRepository) GetUser(ctx context.Context, id int64) (User, error) {
	user, err := r.queries.GetUser(ctx, int32(id))
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return toRepositoryUser(user), nil
}

func (r *SQLCRepository) ListUsers(ctx context.Context) ([]User, error) {
	users, err := r.queries.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]User, 0, len(users))
	for _, user := range users {
		result = append(result, toRepositoryUser(user))
	}
	return result, nil
}

func (r *SQLCRepository) FindByUsername(ctx context.Context, username string) (User, bool, error) {
	user, err := r.queries.GetUserByUsername(ctx, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, false, nil
		}
		return User{}, false, err
	}
	return toRepositoryUser(user), true, nil
}

func (r *SQLCRepository) UpdateUser(ctx context.Context, user User) (User, error) {
	updated, err := r.queries.UpdateUser(ctx, pgsqlc.UpdateUserParams{
		ID:       int32(user.ID),
		Name:     user.Name,
		Email:    user.Email,
		Username: user.Username,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return toRepositoryUser(updated), nil
}

func (r *SQLCRepository) DeleteUser(ctx context.Context, id int64) error {
	rowsAffected, err := r.queries.DeleteUser(ctx, int32(id))
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLCRepository) CreateOrder(ctx context.Context, order Order) (Order, error) {
	id, err := r.queries.CreateOrder(ctx, pgsqlc.CreateOrderParams{
		UserID:      int32(order.UserID),
		Number:      order.Number,
		Status:      order.Status,
		TotalAmount: order.TotalAmount,
	})
	if err != nil {
		return Order{}, err
	}
	created, err := r.queries.GetOrder(ctx, id)
	if err != nil {
		return Order{}, err
	}
	return toRepositoryOrder(created), nil
}

func (r *SQLCRepository) GetOrder(ctx context.Context, id int64) (Order, error) {
	order, err := r.queries.GetOrder(ctx, int32(id))
	if err != nil {
		if err == sql.ErrNoRows {
			return Order{}, ErrNotFound
		}
		return Order{}, err
	}
	return toRepositoryOrder(order), nil
}

func (r *SQLCRepository) ListOrders(ctx context.Context) ([]Order, error) {
	orders, err := r.queries.ListOrders(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Order, 0, len(orders))
	for _, order := range orders {
		result = append(result, toRepositoryOrder(order))
	}
	return result, nil
}

func (r *SQLCRepository) FindByNumber(ctx context.Context, number string) (Order, bool, error) {
	order, err := r.queries.GetOrderByNumber(ctx, number)
	if err != nil {
		if err == sql.ErrNoRows {
			return Order{}, false, nil
		}
		return Order{}, false, err
	}
	return toRepositoryOrder(order), true, nil
}

func (r *SQLCRepository) UpdateOrder(ctx context.Context, order Order) (Order, error) {
	updated, err := r.queries.UpdateOrder(ctx, pgsqlc.UpdateOrderParams{
		ID:          int32(order.ID),
		UserID:      int32(order.UserID),
		Number:      order.Number,
		Status:      order.Status,
		TotalAmount: order.TotalAmount,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return Order{}, ErrNotFound
		}
		return Order{}, err
	}
	return toRepositoryOrder(updated), nil
}

func (r *SQLCRepository) DeleteOrder(ctx context.Context, id int64) error {
	rowsAffected, err := r.queries.DeleteOrder(ctx, int32(id))
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func toRepositoryUser(user pgsqlc.User) User {
	return User{
		ID:       int64(user.ID),
		Name:     user.Name,
		Email:    user.Email,
		Username: user.Username,
	}
}

func toRepositoryOrder(order pgsqlc.Order) Order {
	return Order{
		ID:          int64(order.ID),
		UserID:      int64(order.UserID),
		Number:      order.Number,
		Status:      order.Status,
		TotalAmount: order.TotalAmount,
	}
}
