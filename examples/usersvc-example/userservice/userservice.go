package usersvc

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/go-playground/validator/v10"
	"github.com/remiges-tech/alya/examples/usersvc-example/pg/sqlc-gen"
	"github.com/remiges-tech/alya/wscutils"

	"github.com/remiges-tech/alya/service"
)

const (
	// Error codes and message IDs
	ErrMsgIDInternalErr   = 1002
	ErrCodeInternalErr    = "internal"
	ErrMsgIDBannedDomain  = 1003
	ErrCodeBannedDomain   = "banned_domain"
	ErrMsgIDInvalidFormat = 1004
	ErrCodeInvalidFormat  = "invalid_format"
	ErrMsgIDAlreadyExists = 1005
	ErrCodeAlreadyExists  = "already_exists"

	// Validation constraints
	MinNameLength     = 2
	MaxNameLength     = 50
	MinUsernameLength = 3
	MaxUsernameLength = 30
	MaxEmailLength    = 100
)

/*
Example message templates for different validation errors:
These show how clients should use the vals array and field in their message templates.
The field name is already available in the ErrorMessage.field, so it's not included in vals.

required:    "This field is required"
            No vals needed, use field from ErrorMessage

min:        "Must be at least @<min>@ characters long"
            vals[0] = minimum length

max:        "Cannot be longer than @<max>@ characters"
            vals[0] = maximum length

email:      "Invalid email address: @<value>@"
            vals[0] = invalid email value

alphanum:   "Contains invalid characters: @<value>@"
            vals[0] = invalid value

e164:       "Invalid phone number format: @<value>@"
            vals[0] = invalid phone number

banned_domain: "Email domain is not allowed"
              No vals needed, use field from ErrorMessage
*/

// ValidationMessages maps validation tags to error messages
var ValidationMessages = map[string]string{
	"required":       "This field is required",
	"email":          "Invalid email format",
	"min":            "Must be at least @<min>@ characters long",
	"max":            "Cannot be longer than @<max>@ characters",
	"alphanum":       "Contains invalid characters: @<value>@",
	"e164":           "Invalid phone number format: @<value>@",
	"banned_domain":  "Email domain is not allowed",
	"already_exists": "This value is already taken",
}

type CreateUserRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=50"`
	Email       string `json:"email" validate:"required,email,max=100"`
	Username    string `json:"username" validate:"required,min=3,max=30,alphanum"`
	PhoneNumber string `json:"phone_number" validate:"omitempty,e164"`
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
	validationErrors := wscutils.WscValidate(createUserReq, func(err validator.FieldError) []string {
		switch err.Tag() {
		case "required":
			return []string{} // Field name is already in ErrorMessage.field
		case "min":
			switch err.Field() {
			case "Name":
				return []string{fmt.Sprintf("%d", MinNameLength)}
			case "Username":
				return []string{fmt.Sprintf("%d", MinUsernameLength)}
			default:
				return []string{err.Param()}
			}
		case "max":
			switch err.Field() {
			case "Name":
				return []string{fmt.Sprintf("%d", MaxNameLength)}
			case "Username":
				return []string{fmt.Sprintf("%d", MaxUsernameLength)}
			case "Email":
				return []string{fmt.Sprintf("%d", MaxEmailLength)}
			default:
				return []string{err.Param()}
			}
		case "email":
			return []string{err.Value().(string)}
		case "alphanum":
			return []string{err.Value().(string)}
		case "e164":
			return []string{err.Value().(string)}
		default:
			return []string{}
		}
	})

	// Custom validation for banned email domains
	if isEmailDomainBanned(createUserReq.Email) {
		validationErrors = append(validationErrors, wscutils.BuildErrorMessage(ErrMsgIDBannedDomain, ErrCodeBannedDomain, "email", ValidationMessages["banned_domain"]))
	}

	// TODO: Add username existence check once SQLC query is generated
	// exists, err := s.Database.(*sqlc.Queries).CheckUsernameExists(c.Request.Context(), createUserReq.Username)
	// if err != nil {
	// 	wscutils.SendErrorResponse(c, wscutils.NewErrorResponse(ErrMsgIDInternalErr, ErrCodeInternalErr))
	// 	return
	// }
	// if exists {
	// 	validationErrors = append(validationErrors, wscutils.BuildErrorMessage(ErrMsgIDAlreadyExists, ErrCodeAlreadyExists, "username", ValidationMessages["already_exists"]))
	// }

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
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false // Malformed email will be caught by email validator
	}

	bannedDomains := []string{"banned.com", "example.com"}
	emailDomain := strings.ToLower(parts[1])
	for _, domain := range bannedDomains {
		if strings.ToLower(domain) == emailDomain {
			return true
		}
	}
	return false
}
