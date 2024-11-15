package usersvc

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/go-playground/validator/v10"
	"github.com/remiges-tech/alya/examples/pg/sqlc-gen"
	"github.com/remiges-tech/alya/wscutils"

	"github.com/remiges-tech/alya/service"
)

const (
	ErrMsgIDInternalErr  = 1002
	ErrCodeInternalErr   = "internal"      // Example error code for invalid input
	ErrMsgIDBannedDomain = 1003            // New custom message ID
	ErrCodeBannedDomain  = "banned_domain" // New custom error code
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
	// Custom validation for banned email domains
	if isEmailDomainBanned(createUserReq.Email) {
		validationErrors = append(validationErrors, wscutils.BuildErrorMessage(ErrMsgIDBannedDomain, ErrCodeBannedDomain, "email"))
	}
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

// Helper function to check if email domain is banned
func isEmailDomainBanned(email string) bool {
	bannedDomains := []string{"banned.com", "example.com"}
	emailDomain := strings.Split(email, "@")[1]
	for _, domain := range bannedDomains {
		if emailDomain == domain {
			return true
		}
	}
	return false
}
