package transport

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/api"
	"github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/repository"
	"github.com/remiges-tech/alya/examples/wsc-usersvc-sqlc-example/internal/service"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
)

type UserHandler struct {
	app       *app.UserService
	validator *wscutils.Validator
	logger    *logharbour.Logger
}

func NewUserHandler(appService *app.UserService, validator *wscutils.Validator, logger *logharbour.Logger) *UserHandler {
	return &UserHandler{app: appService, validator: validator, logger: logger}
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	h.logger.Info().LogActivity("CreateUser request received", nil)

	var req api.CreateUserRequest
	if err := wscutils.BindData(c, &req); err != nil {
		sendError(c, http.StatusBadRequest, invalidJSONMessages())
		return
	}
	if errs := h.validator.Validate(req); len(errs) > 0 {
		sendError(c, http.StatusBadRequest, errs)
		return
	}

	user, err := h.app.CreateUser(c.Request.Context(), req)
	if err != nil {
		status, messages := h.userAppError(err)
		sendError(c, status, messages)
		return
	}
	sendSuccess(c, toUserResponse(user))
}

func (h *UserHandler) ListUsers(c *gin.Context) {
	h.logger.Info().LogActivity("ListUsers request received", nil)

	users, err := h.app.ListUsers(c.Request.Context())
	if err != nil {
		h.logger.Error(fmt.Errorf("list users failed: %w", err)).LogActivity("Request failed", nil)
		sendError(c, http.StatusInternalServerError, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalid, "internal", "")})
		return
	}
	response := make([]api.UserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, toUserResponse(user))
	}
	sendSuccess(c, response)
}

func (h *UserHandler) GetUser(c *gin.Context) {
	h.logger.Info().LogActivity("GetUser request received", map[string]any{"id": c.Param("id")})

	id, err := wscutils.ParseInt64PathParam(c, "id")
	if err != nil {
		sendError(c, http.StatusBadRequest, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "id")})
		return
	}
	user, err := h.app.GetUser(c.Request.Context(), id)
	if err != nil {
		status, messages := h.userAppError(err)
		sendError(c, status, messages)
		return
	}
	sendSuccess(c, toUserResponse(user))
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	h.logger.Info().LogActivity("UpdateUser request received", map[string]any{"id": c.Param("id")})

	id, err := wscutils.ParseInt64PathParam(c, "id")
	if err != nil {
		sendError(c, http.StatusBadRequest, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "id")})
		return
	}

	var req api.UpdateUserRequest
	if err := wscutils.BindData(c, &req); err != nil {
		sendError(c, http.StatusBadRequest, invalidJSONMessages())
		return
	}
	if errs := validateUpdateUserRequest(req, h.validator); len(errs) > 0 {
		sendError(c, http.StatusBadRequest, errs)
		return
	}

	user, err := h.app.UpdateUser(c.Request.Context(), id, req)
	if err != nil {
		status, messages := h.userAppError(err)
		sendError(c, status, messages)
		return
	}
	sendSuccess(c, toUserResponse(user))
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	h.logger.Info().LogActivity("DeleteUser request received", map[string]any{"id": c.Param("id")})

	id, err := wscutils.ParseInt64PathParam(c, "id")
	if err != nil {
		sendError(c, http.StatusBadRequest, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "id")})
		return
	}
	if err := h.app.DeleteUser(c.Request.Context(), id); err != nil {
		status, messages := h.userAppError(err)
		sendError(c, status, messages)
		return
	}
	sendSuccess(c, map[string]any{})
}

func validateUpdateUserRequest(req api.UpdateUserRequest, validator *wscutils.Validator) []wscutils.ErrorMessage {
	var errs []wscutils.ErrorMessage
	if req.Name.Null {
		errs = append(errs, wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "name"))
	}
	if req.Email.Null {
		errs = append(errs, wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "email"))
	}
	if req.Username.Null {
		errs = append(errs, wscutils.BuildErrorMessage(msgIDInvalid, "invalid", "username"))
	}
	if value, ok := req.Name.Get(); ok {
		tmp := struct {
			Name string `json:"name" validate:"min=2,max=50"`
		}{Name: value}
		errs = append(errs, validator.Validate(tmp)...)
	}
	if value, ok := req.Email.Get(); ok {
		tmp := struct {
			Email string `json:"email" validate:"email,max=100"`
		}{Email: value}
		errs = append(errs, validator.Validate(tmp)...)
	}
	if value, ok := req.Username.Get(); ok {
		tmp := struct {
			Username string `json:"username" validate:"min=3,max=30,alphanum"`
		}{Username: value}
		errs = append(errs, validator.Validate(tmp)...)
	}
	return errs
}

func (h *UserHandler) userAppError(err error) (int, []wscutils.ErrorMessage) {
	switch err {
	case app.ErrUserNotFound:
		return http.StatusNotFound, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDNotFound, "missing", "id")}
	case app.ErrUsernameExists:
		return http.StatusConflict, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDConflict, "exists", "username")}
	default:
		h.logger.Error(fmt.Errorf("user request failed: %w", err)).LogActivity("Request failed", nil)
		return http.StatusInternalServerError, []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalid, "internal", "")}
	}
}

func invalidJSONMessages() []wscutils.ErrorMessage {
	return []wscutils.ErrorMessage{wscutils.BuildErrorMessage(msgIDInvalidJSON, errCodeInvalidJSON, "")}
}

func toUserResponse(user repository.User) api.UserResponse {
	return api.UserResponse{ID: user.ID, Name: user.Name, Email: user.Email, Username: user.Username}
}
