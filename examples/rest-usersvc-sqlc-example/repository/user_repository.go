package repository

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("not found")

type User struct {
	ID       int64
	Name     string
	Email    string
	Username string
}

type UserRepository interface {
	CreateUser(ctx context.Context, user User) (User, error)
	GetUser(ctx context.Context, id int64) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	FindByUsername(ctx context.Context, username string) (User, bool, error)
	UpdateUser(ctx context.Context, user User) (User, error)
	DeleteUser(ctx context.Context, id int64) error
}
