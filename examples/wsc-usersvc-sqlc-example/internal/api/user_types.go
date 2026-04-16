package api

import "github.com/remiges-tech/alya/wscutils"

type CreateUserRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=50"`
	Email    string `json:"email" validate:"required,email,max=100"`
	Username string `json:"username" validate:"required,min=3,max=30,alphanum"`
}

type UpdateUserRequest struct {
	Name     wscutils.Optional[string] `json:"name"`
	Email    wscutils.Optional[string] `json:"email"`
	Username wscutils.Optional[string] `json:"username"`
}

type UserResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}
