package usersvc

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/remiges-tech/alya/examples/usersvc-example/pg/sqlc-gen"
	"github.com/remiges-tech/alya/service"
	"github.com/remiges-tech/alya/wscutils"
)

//-----------------------------------------------------------------------------
// Constants
//-----------------------------------------------------------------------------

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

//-----------------------------------------------------------------------------
// Message Templates Documentation
//-----------------------------------------------------------------------------

/*
Example message templates for different validation errors:
These show how clients should use the vals array and field in their message templates.
The field name is already available in the ErrorMessage.field, so it's not included in vals.

required:    "This field is required"
            No vals needed, use field from ErrorMessage

length:     "The field length (@<vals[0]>@) must be between @<vals[1]>@ and @<vals[2]>@ characters"
            vals[0] = current length
            vals[1] = minimum length
            vals[2] = maximum length
            Example for Name field: "The field length (1) must be between 2 and 50 characters"

email:      "Invalid email address: @<value>@"
            vals[0] = invalid email value

alphanum:   "Contains invalid characters: @<value>@"
            vals[0] = invalid value

e164:       "Invalid phone number format: @<value>@"
            vals[0] = invalid phone number

banned_domain: "Email domain is not allowed"
              No vals needed, use field from ErrorMessage
*/

//-----------------------------------------------------------------------------
// Validation Messages
//-----------------------------------------------------------------------------

var ValidationMessages = map[string]string{
	"required":      "This field is required",
	"email":         "Invalid email format",
	"length_error":  "The field length (@<vals[0]>@) must be between @<vals[1]>@ and @<vals[2]>@ characters",
	"alphanum":      "Contains invalid characters: @<value>@",
	"e164":          "Invalid phone number format: @<value>@",
	"banned_domain": "Email domain is not allowed",
}

//-----------------------------------------------------------------------------
// Request Types
//-----------------------------------------------------------------------------

type CreateUserRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=50"`
	Email       string `json:"email" validate:"required,email,max=100"`
	Username    string `json:"username" validate:"required,min=3,max=30,alphanum"`
	PhoneNumber string `json:"phone_number" validate:"omitempty,e164"`
}

//-----------------------------------------------------------------------------
// Initialization
//-----------------------------------------------------------------------------

func init() {
	// Step 1: Set up validation tag to error code mapping
	wscutils.SetValidationTagToErrCodeMap(map[string]string{
		"required": "required",
		"min":      "length_error",
		"max":      "length_error",
		"email":    "invalid_format",
		"alphanum": "invalid_format",
		"e164":     "invalid_format",
	})

	// Step 2: Set up validation tag to message ID mapping
	wscutils.SetValidationTagToMsgIDMap(map[string]int{
		"required": 101,
		"min":      101,
		"max":      101,
		"email":    101,
		"alphanum": 101,
		"e164":     101,
	})

	// Step 3: Set default error code and message ID
	wscutils.SetDefaultErrCode("invalid_format")
	wscutils.SetDefaultMsgID(101)
}

//-----------------------------------------------------------------------------
// Request Handlers
//-----------------------------------------------------------------------------

func HandleCreateUserRequest(c *gin.Context, s *service.Service) {
	s.LogHarbour.Log("CreateUser request received")

	// Get queries object once at the start
	queries := s.Database.(*sqlc.Queries)

	// Get validation constraints from Rigel
	minNameLength, err := s.RigelConfig.Get(c.Request.Context(), "validation.name.minLength")
	if err != nil {
		minNameLength = "2" // Default value
	}
	maxNameLength, err := s.RigelConfig.Get(c.Request.Context(), "validation.name.maxLength")
	if err != nil {
		maxNameLength = "50" // Default value
	}
	minUsernameLength, err := s.RigelConfig.Get(c.Request.Context(), "validation.username.minLength")
	if err != nil {
		minUsernameLength = "3" // Default value
	}
	maxUsernameLength, err := s.RigelConfig.Get(c.Request.Context(), "validation.username.maxLength")
	if err != nil {
		maxUsernameLength = "30" // Default value
	}
	maxEmailLength, err := s.RigelConfig.Get(c.Request.Context(), "validation.email.maxLength")
	if err != nil {
		maxEmailLength = "100" // Default value
	}

	//-------------------------------------------------------------------------
	// Step 1: Parse and bind request data
	//-------------------------------------------------------------------------
	var createUserReq CreateUserRequest
	if err := wscutils.BindJSON(c, &createUserReq); err != nil {
		return
	}
	s.LogHarbour.Log(fmt.Sprintf("CreateUser request parsed: %v", map[string]any{"username": createUserReq.Name}))

	//-------------------------------------------------------------------------
	// Step 2: Validate request data
	//-------------------------------------------------------------------------
	validationErrors := wscutils.WscValidate(createUserReq, func(err validator.FieldError) []string {
		switch err.Tag() {
		case "required":
			return []string{} // Field name is already in ErrorMessage.field

		case "min", "max":
			currentLen := len(err.Value().(string))
			switch err.Field() {
			case "Name":
				return []string{fmt.Sprintf("%d", currentLen), minNameLength, maxNameLength}
			case "Username":
				return []string{fmt.Sprintf("%d", currentLen), minUsernameLength, maxUsernameLength}
			case "Email":
				return []string{fmt.Sprintf("%d", currentLen), "0", maxEmailLength}
			default:
				return []string{fmt.Sprintf("%d", currentLen), "0", err.Param()}
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

	if len(validationErrors) > 0 {
		c.JSON(400, wscutils.NewResponse("error", nil, validationErrors))
		return
	}

	//-------------------------------------------------------------------------
	// Step 3: Perform business rule validations
	//-------------------------------------------------------------------------
	if isEmailDomainBanned(createUserReq.Email) {
		bannedDomainError := wscutils.BuildErrorMessage(ErrMsgIDBannedDomain, ErrCodeBannedDomain, "Email")
		c.JSON(400, wscutils.NewResponse("error", nil, []wscutils.ErrorMessage{bannedDomainError}))
		return
	}

	//-------------------------------------------------------------------------
	// Step 4: Check data dependencies
	//-------------------------------------------------------------------------
	exists, err := queries.CheckUsernameExists(c.Request.Context(), createUserReq.Username)
	if err != nil {
		s.LogHarbour.Error(fmt.Errorf("error checking existing user: %w", err))
		c.JSON(500, wscutils.NewErrorResponse(ErrMsgIDInternalErr, ErrCodeInternalErr))
		return
	}
	if exists {
		alreadyExistsError := wscutils.BuildErrorMessage(ErrMsgIDAlreadyExists, ErrCodeAlreadyExists, "Username")
		c.JSON(400, wscutils.NewResponse("error", nil, []wscutils.ErrorMessage{alreadyExistsError}))
		return
	}

	//-------------------------------------------------------------------------
	// Step 5: Perform core business logic
	//-------------------------------------------------------------------------
	user, err := queries.CreateUser(c.Request.Context(), sqlc.CreateUserParams{
		Name:        createUserReq.Name,
		Email:       createUserReq.Email,
		Username:    createUserReq.Username,
		PhoneNumber: sql.NullString{String: createUserReq.PhoneNumber, Valid: createUserReq.PhoneNumber != ""},
	})
	if err != nil {
		s.LogHarbour.Error(fmt.Errorf("error creating user: %w", err))
		c.JSON(500, wscutils.NewErrorResponse(ErrMsgIDInternalErr, ErrCodeInternalErr))
		return
	}

	//-------------------------------------------------------------------------
	// Step 6: Send response
	//-------------------------------------------------------------------------
	wscutils.SendSuccessResponse(c, wscutils.NewSuccessResponse(user))
}

//-----------------------------------------------------------------------------
// Helper Functions
//-----------------------------------------------------------------------------

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
