package transport

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/api"
	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/app"
	"github.com/remiges-tech/alya/examples/rest-usersvc-sqlc-example/repository"
	"github.com/remiges-tech/alya/restutils"
	"github.com/remiges-tech/alya/service"
	"github.com/remiges-tech/alya/wscutils"
)

type UserHandler struct {
	app       *app.UserService
	validator *restutils.Validator
}

func NewUserHandler(appService *app.UserService, validator *restutils.Validator) *UserHandler {
	return &UserHandler{app: appService, validator: validator}
}

func (h *UserHandler) CreateUser(c *gin.Context, _ *service.Service) {
	var req api.CreateUserRequest
	if err := restutils.BindBody(c, &req); err != nil {
		restutils.WriteProblem(c, restutils.ProblemFromBindError(err))
		return
	}
	if errs := h.validator.Validate(req); len(errs) > 0 {
		restutils.WriteProblem(c, restutils.ValidationProblem(errs))
		return
	}

	user, err := h.app.CreateUser(c.Request.Context(), req)
	if err != nil {
		restutils.WriteProblem(c, problemFromAppError(err))
		return
	}
	location := fmt.Sprintf("/users/%d", user.ID)
	restutils.WriteCreated(c, location, toUserResponse(user))
}

func (h *UserHandler) ListUsers(c *gin.Context, _ *service.Service) {
	users, err := h.app.ListUsers(c.Request.Context())
	if err != nil {
		restutils.WriteProblem(c, restutils.InternalServerError())
		return
	}
	response := make([]api.UserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, toUserResponse(user))
	}
	restutils.WriteOK(c, response)
}

func (h *UserHandler) GetUser(c *gin.Context, _ *service.Service) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		restutils.WriteProblem(c, badRequestProblem("invalid user id"))
		return
	}
	user, err := h.app.GetUser(c.Request.Context(), id)
	if err != nil {
		restutils.WriteProblem(c, problemFromAppError(err))
		return
	}
	restutils.WriteOK(c, toUserResponse(user))
}

func (h *UserHandler) UpdateUser(c *gin.Context, _ *service.Service) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		restutils.WriteProblem(c, badRequestProblem("invalid user id"))
		return
	}

	var req api.UpdateUserRequest
	if err := restutils.BindBody(c, &req); err != nil {
		restutils.WriteProblem(c, restutils.ProblemFromBindError(err))
		return
	}
	if errs := validateUpdateRequest(req, h.validator); len(errs) > 0 {
		restutils.WriteProblem(c, restutils.ValidationProblem(errs))
		return
	}

	user, err := h.app.UpdateUser(c.Request.Context(), id, req)
	if err != nil {
		restutils.WriteProblem(c, problemFromAppError(err))
		return
	}
	restutils.WriteOK(c, toUserResponse(user))
}

func (h *UserHandler) DeleteUser(c *gin.Context, _ *service.Service) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		restutils.WriteProblem(c, badRequestProblem("invalid user id"))
		return
	}
	if err := h.app.DeleteUser(c.Request.Context(), id); err != nil {
		restutils.WriteProblem(c, problemFromAppError(err))
		return
	}
	restutils.WriteNoContent(c)
}

func parseID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}

func validateUpdateRequest(req api.UpdateUserRequest, validator *restutils.Validator) []restutils.FieldError {
	var fieldErrors []restutils.FieldError

	if req.Name.Null {
		fieldErrors = append(fieldErrors, restutils.FieldError{ErrorMessage: wscutils.BuildErrorMessage(104, "invalid", "name"), Message: "must not be null"})
	}
	if req.Email.Null {
		fieldErrors = append(fieldErrors, restutils.FieldError{ErrorMessage: wscutils.BuildErrorMessage(104, "invalid", "email"), Message: "must not be null"})
	}
	if req.Username.Null {
		fieldErrors = append(fieldErrors, restutils.FieldError{ErrorMessage: wscutils.BuildErrorMessage(104, "invalid", "username"), Message: "must not be null"})
	}
	if value, ok := req.Name.Get(); ok {
		tmp := struct {
			Name string `json:"name" validate:"min=2,max=50"`
		}{Name: value}
		fieldErrors = append(fieldErrors, validator.Validate(tmp)...)
	}
	if value, ok := req.Email.Get(); ok {
		tmp := struct {
			Email string `json:"email" validate:"email,max=100"`
		}{Email: value}
		fieldErrors = append(fieldErrors, validator.Validate(tmp)...)
	}
	if value, ok := req.Username.Get(); ok {
		tmp := struct {
			Username string `json:"username" validate:"min=3,max=30,alphanum"`
		}{Username: value}
		fieldErrors = append(fieldErrors, validator.Validate(tmp)...)
	}
	return fieldErrors
}

func problemFromAppError(err error) restutils.Problem {
	switch {
	case errors.Is(err, app.ErrUserNotFound):
		return restutils.NewProblem(http.StatusNotFound, "https://alya.dev/problems/not-found", "Resource not found", "user was not found")
	case errors.Is(err, app.ErrUsernameExists):
		return restutils.NewProblem(http.StatusConflict, "https://alya.dev/problems/conflict", "Conflict", "username already exists")
	default:
		return restutils.InternalServerError()
	}
}

func badRequestProblem(detail string) restutils.Problem {
	return restutils.NewProblem(http.StatusBadRequest, "https://alya.dev/problems/bad-request", "Bad request", detail)
}

func toUserResponse(user repository.User) api.UserResponse {
	return api.UserResponse{
		ID:       user.ID,
		Name:     user.Name,
		Email:    user.Email,
		Username: user.Username,
	}
}
