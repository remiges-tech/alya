package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

type User struct {
	Fullname string `json:"fullname" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Age      int    `json:"age" validate:"min=10,max=150"`
}

func main() {
	router := gin.Default()

	router.POST("/user", func(c *gin.Context) {
		var user User
		err := c.BindJSON(&user)
		// this will go away once we have a middleware or API gateway that does this
		if err != nil {
			invalidJsonError := BuildValidationError("", "invalid_json")

			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"data":   gin.H{},
				"messages": []WscValidationError{
					invalidJsonError,
				}})
			return
		}

		validationErrors := validate(user)

		// add validation errors to response and send response
		if len(validationErrors) > 0 {
			c.JSON(400, gin.H{
				"status":   "error",
				"data":     gin.H{},
				"messages": validationErrors,
			})
			return
		}

		// send success response
		c.JSON(200, gin.H{
			"status":   "success",
			"data":     user,
			"messages": []string{},
		})
	})

	router.Run(":8080")
}

// validate validates the request body
func validate(user User) []WscValidationError {
	validationErrors := WscValidate(user)

	// add request-specific vals to validation errors
	validationErrors = addVals(validationErrors, user)

	// check request specific custom validations and add errors
	validationErrors = addCustomValidationErrors(validationErrors, user)

	return validationErrors
}

// addVals adds request-specific values to a slice of WscValidationError returned by standard validator
// This is required because vals for different requests could be different.
func addVals(validationErrors []WscValidationError, user User) []WscValidationError {
	for i, err := range validationErrors {
		switch err.Field {
		case "Age":
			inputValue := fmt.Sprintf("%v", user.Age)
			validValue := "10-150"
			validationErrors[i].AddVals([]string{inputValue, validValue})
		case "Fullname":
			inputValue := "Not provided"
			validationErrors[i].AddVals([]string{inputValue})
		case "Email":
			if err.Code == "required" {
				inputValue := "Not provided"
				validationErrors[i].AddVals([]string{inputValue})
			} else if err.Code == "email" {
				inputValue := "Invalid format"
				validationErrors[i].AddVals([]string{inputValue})
			}
		}
	}

	return validationErrors
}

// addCustomValidationErrors adds custom validation errors to the validationErrors slice.
// This is required because request specific custom validators are not supported by wscvalidation.
func addCustomValidationErrors(validationErrors []WscValidationError, user User) []WscValidationError {
	// Example of a custom validation for email domain
	if user.Email != "" && !strings.Contains(user.Email, "@domain.com") {
		emailDomainError := BuildValidationError("Email", "emaildomain")
		emailDomainError.AddVals([]string{user.Email, "@domain.com"})
		validationErrors = append(validationErrors, emailDomainError)
	}

	return validationErrors
}
