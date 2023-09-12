package user

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"go-framework/internal/wscutils"
	"net/http"
	"strings"
)

type User struct {
	Fullname string `json:"fullname" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Age      int    `json:"age" validate:"min=10,max=150"`
}

// RegisterUserHandlers registers all the user-related handlers
func RegisterUserHandlers(router *gin.Engine) { // we create a function to register all the user-related handlers
	router.POST("/user", createUser)
	// router.GET("/user", getUser)
	// other user specific handlers
}

// createUser handles the POST /user request
func createUser(c *gin.Context) {
	var user User

	// step 1: bind request body to struct
	if err := wscutils.BindJson(c, &user); err != nil {
		return
	}

	// step 2: validate request body
	validationErrors := validate(user)

	// step 3: if there are validation errors, add them to response and send it
	if len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, validationErrors))
		return
	}

	// step 4: if there are no validation errors, send success response
	c.JSON(http.StatusOK, wscutils.NewResponse(wscutils.SuccessStatus, &user, []wscutils.ErrorMessage{}))
}

// validate validates the request body
func validate(user User) []wscutils.ErrorMessage {
	// step 2.1: validate request body using standard validator
	validationErrors := wscutils.WscValidate(user)

	// step 2.2: add request-specific vals to validation errors
	// NOTE: it mutates validationErrors
	validationErrors = addVals(validationErrors, user)

	// if there are standard validation errors, return
	// do not execute custom validations
	if len(validationErrors) > 0 {
		return validationErrors
	}

	// step 2.3: check request specific custom validations and add errors
	validationErrors = addCustomValidationErrors(validationErrors, user)

	return validationErrors
}

// addVals adds request-specific values to a slice of ErrorMessage returned by standard validator
// This is required because vals for different requests could be different.
func addVals(validationErrors []wscutils.ErrorMessage, user User) []wscutils.ErrorMessage {
	for i, err := range validationErrors {
		switch err.Field {
		case UserAge:
			inputValue := fmt.Sprintf("%v", user.Age)
			validValue := AgeRange
			validationErrors[i].Vals = []string{inputValue, validValue}
		case UserFullname:
			inputValue := NotProvided
			validationErrors[i].Vals = []string{inputValue}
		case UserEmail:
			if err.Code == wscutils.RequiredError {
				inputValue := NotProvided
				validationErrors[i].Vals = []string{inputValue}
			} else if err.Code == wscutils.InvalidEmail {
				inputValue := user.Email
				validationErrors[i].Vals = []string{inputValue}
			}
		}
	}

	return validationErrors
}

// addCustomValidationErrors adds custom validation errors to the validationErrors slice.
// This is required because request specific custom validators are not supported by wscvalidation.
func addCustomValidationErrors(validationErrors []wscutils.ErrorMessage, user User) []wscutils.ErrorMessage {
	// Example of a custom validation for email domain
	if user.Email != "" && !strings.Contains(user.Email, "@domain.com") {
		emailDomainError := wscutils.BuildValidationError(UserEmail, wscutils.EmailDomainErr)
		emailDomainError.Vals = []string{user.Email, "@domain.com"}
		validationErrors = append(validationErrors, emailDomainError)
	}

	return validationErrors
}
