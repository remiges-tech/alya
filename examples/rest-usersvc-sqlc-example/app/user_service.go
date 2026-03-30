package app

import (
	"context"
	"errors"

	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/api"
	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/repository"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrUsernameExists = errors.New("username already exists")
)

type UserService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) CreateUser(ctx context.Context, req api.CreateUserRequest) (repository.User, error) {
	if _, exists, err := s.repo.FindByUsername(ctx, req.Username); err != nil {
		return repository.User{}, err
	} else if exists {
		return repository.User{}, ErrUsernameExists
	}

	return s.repo.CreateUser(ctx, repository.User{
		Name:     req.Name,
		Email:    req.Email,
		Username: req.Username,
	})
}

func (s *UserService) GetUser(ctx context.Context, id int64) (repository.User, error) {
	user, err := s.repo.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.User{}, ErrUserNotFound
		}
		return repository.User{}, err
	}
	return user, nil
}

func (s *UserService) ListUsers(ctx context.Context) ([]repository.User, error) {
	return s.repo.ListUsers(ctx)
}

func (s *UserService) UpdateUser(ctx context.Context, id int64, req api.UpdateUserRequest) (repository.User, error) {
	user, err := s.repo.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.User{}, ErrUserNotFound
		}
		return repository.User{}, err
	}

	if name, ok := req.Name.Get(); ok {
		user.Name = name
	}
	if email, ok := req.Email.Get(); ok {
		user.Email = email
	}
	if username, ok := req.Username.Get(); ok {
		if username != user.Username {
			if _, exists, err := s.repo.FindByUsername(ctx, username); err != nil {
				return repository.User{}, err
			} else if exists {
				return repository.User{}, ErrUsernameExists
			}
		}
		user.Username = username
	}

	updatedUser, err := s.repo.UpdateUser(ctx, user)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.User{}, ErrUserNotFound
		}
		return repository.User{}, err
	}
	return updatedUser, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id int64) error {
	if err := s.repo.DeleteUser(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	return nil
}
