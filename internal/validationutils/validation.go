// Validationutils provides a set of utilities for validating
// and format the validation errors for API requests.
package main

import (
	"github.com/asaskevich/govalidator"
	"strings"
)

// ValidationError is a struct that defines the format
// of all validation error responses from the server.
// This is designed for uniformity of server response and easy
// consumption by the client regardless of the type of validation error.
type ValidationError struct {
	Code    string `json:"code"`
	Msgcode int    `json:"msgcode"`
	Field   string `json:"field"`
}

// Validate is a generic function that accepts any data structure,
// validates it according to struct tag-provided validation rules
// and returns a slice of ValidationError in case of validation errors.
// If there are no validation errors, it returns an empty slice.
// This design allows for usage across different data structures
// and validation rules in a uniform manner.
func Validate(data interface{}) []ValidationError {
	var validationErrors []ValidationError

	// govalidator.ValidateStruct takes our data input
	// and validates it according to the provided struct tags.
	valid, err := govalidator.ValidateStruct(data)

	if !valid {
		errs := err.(govalidator.Errors)

		for _, e := range errs.Errors() {
			validationError := ValidationError{}
			errorString := e.Error()

			// The error messages returned by govalidator typically
			// take the form 'name: validationmessage', hence we split this
			// to extract field name and error type.
			splitErr := strings.Split(errorString, ":")
			fieldName := strings.TrimSpace(splitErr[0])
			errorType := strings.TrimSpace(splitErr[1])

			// Map govalidator's validation
			// rule failures to our custom error codes and messages.
			switch errorType {
			case "required":
				validationError = ValidationError{"missing", 45, fieldName}
			case "email":
				validationError = ValidationError{"invalid", 50, fieldName}
			case "range":
				validationError = ValidationError{"outofrange", 55, fieldName}
			default:
				validationError = ValidationError{"unknown", 0, fieldName}
			}

			validationErrors = append(validationErrors, validationError)
		}
	}

	return validationErrors
}
