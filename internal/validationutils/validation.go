// Validationutils provides a set of utilities for validating request bodies
// and formatting the validation errors
package main

import (
	"errors"
	"github.com/go-playground/validator/v10"
)

// ValidationError is a struct that defines the format
// of error part of the standard response object specified at
// See: https://redmine.bquanta.xyz/projects/mail-doc/wiki/Websvcgeneral#Web-service-response-format
type ValidationError struct {
	Field   string   `json:"field"`
	Code    string   `json:"code"`
	Msgcode int      `json:"msgcode"`
	Vals    []string `json:"vals,omitempty"` // omit if Vals is empty
}

// FieldValuesFunc retrieves both the input value and validated value for
// a field from a data structure. This is required because this package
// is not aware of the data structures being validated.
// The caller of Validate would implement the logic to get field's input
// value and allowed values for the validation rule for each field.
// These values go into `vals` field or the error response.
// See: https://redmine.bquanta.xyz/projects/mail-doc/wiki/Websvcgeneral#Web-service-response-format
type FieldValuesFunc[T any] func(data T, fieldName string) []string

// Validate is a generic function that accepts any data structure,
// validates it according to struct tag-provided validation rules
// and returns a slice of ValidationError in case of validation errors.
// This design allows validating reqeust bodies in a uniform manner.
func Validate[T any](data T, getValuesFunc FieldValuesFunc[T]) []ValidationError {
	var validationErrors []ValidationError

	validate := validator.New()

	err := validate.Struct(data)

	if err != nil {
		var errs validator.ValidationErrors
		if errors.As(err, &errs) {
			for _, e := range errs {
				fieldName := e.Field()
				errorType := e.Tag()

				// vals array from https://redmine.bquanta.xyz/projects/mail-doc/wiki/Websvcgeneral#Web-service-response-format
				// The caller knows the logic to get these values for their data structure.
				vals := getValuesFunc(data, fieldName)

				validationError := GetValidationError(fieldName, errorType, vals)

				validationErrors = append(validationErrors, validationError)
			}
		}
	}

	return validationErrors
}
