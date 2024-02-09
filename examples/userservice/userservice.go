package usersvc

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/go-playground/validator/v10"
	"github.com/remiges-tech/alya/examples/pg/sqlc-gen"
	"github.com/remiges-tech/alya/wscutils"

	"github.com/remiges-tech/alya/service"
)

const (
	ErrMsgIDInternalErr = 1002
	ErrCodeInternalErr  = "internal" // Example error code for invalid input
)

type CreateUserRequest struct {
	Name        string `json:"name" validate:"required"`
	Email       string `json:"email" validate:"required,email"`
	Username    string `json:"username" validate:"required,min=3"`
	PhoneNumber string `json:"phone_number" validate:"omitempty,e164"` // Assuming e164 format for phone numbers
}

func HandleCreateUserRequest(c *gin.Context, s *service.Service) {
	s.LogHarbour.Log("CreateUser request received")
	// Step 1: Parse request
	var createUserReq CreateUserRequest
	if err := wscutils.BindJSON(c, &createUserReq); err != nil {
		return
	}
	s.LogHarbour.Log(fmt.Sprintf("CreateUser request parsed: %v", map[string]any{"username": createUserReq.Name}))

	// Step 2: Validate request
	validationErrors := wscutils.WscValidate(createUserReq, func(err validator.FieldError) []string { return []string{} })
	if len(validationErrors) > 0 {
		wscutils.SendErrorResponse(c, wscutils.NewResponse(wscutils.ErrorStatus, nil, validationErrors))
		return
	}
	s.LogHarbour.Log(fmt.Sprintf("CreateUser request validated %v", map[string]any{"username": createUserReq.Name}))

	// Step 3: Process data
	// Call CreateUser function
	user, err := s.Database.(*sqlc.Queries).CreateUser(c.Request.Context(), sqlc.CreateUserParams{
		Name:  createUserReq.Name,
		Email: createUserReq.Email,
	})
	if err != nil {
		wscutils.SendErrorResponse(c, wscutils.NewErrorResponse(ErrMsgIDInternalErr, ErrCodeInternalErr))
		return
	}
	s.LogHarbour.Log(fmt.Sprintf("User created: %v", map[string]any{"username": createUserReq.Name}))

	// Step 4: Send response
	wscutils.SendSuccessResponse(c, wscutils.NewSuccessResponse(user))
}
