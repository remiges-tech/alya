// Validationutils provides a set of utilities for validating request bodies
// and formatting the validation errors
package main

import (
	"encoding/json"
	"errors"
	"github.com/go-playground/validator/v10"
	"io"
	"log"
	"os"
)

// WscValidationError is a struct that defines the format
// of error part of the standard response object specified at
// See: https://redmine.bquanta.xyz/projects/mail-doc/wiki/Websvcgeneral#Web-service-response-format
type WscValidationError struct {
	Field   string   `json:"field"`
	Code    string   `json:"code"`
	Msgcode int      `json:"msgcode"`
	Vals    []string `json:"vals,omitempty"` // omit if Vals is empty
}

type errorAttrs struct {
	Code    string `json:"code"`
	MsgCode int    `json:"msgcode"`
}

// errorTypes is a map from error type to its corresponding attributes.
var errorTypes map[string]errorAttrs

func init() {
	errorTypes = make(map[string]errorAttrs)
	loadErrorTypes()
}

func loadErrorTypes() {
	jsonFile, err := os.Open("errortypes.yaml")
	if err != nil {
		log.Panic(err)
	}
	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}(jsonFile)

	byteValue, _ := io.ReadAll(jsonFile)

	err = json.Unmarshal(byteValue, &errorTypes)
	if err != nil {
		log.Panic(err)
	}
}

// WscValidate is a generic function that accepts any data structure,
// validates it according to struct tag-provided validation rules
// and returns a slice of WscValidationError in case of validation errors.
// This design allows validating reqeust bodies in a uniform manner.
// This function will not add `vals` that's required as per the specifications
// because it does not know the request-specific values.
// `vals` will be added to WscValidationError by the caller.
func WscValidate[T any](data T) []WscValidationError {
	var validationErrors []WscValidationError

	validate := validator.New()

	err := validate.Struct(data)

	if err != nil {
		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			for _, err := range validationErrs {
				// We handle validation error creation for developers.
				vErr := BuildValidationError(err.Field(), err.Tag())
				validationErrors = append(validationErrors, vErr)
			}
		}
	}
	return validationErrors
}

// BuildValidationError generates a WscValidationError which includes
// the required validation error information such as code, msgcode
func BuildValidationError(fieldName, errorType string) WscValidationError {
	errorAttr, exists := errorTypes[errorType]
	if !exists {
		errorAttr = errorTypes["unknown"]
	}

	return WscValidationError{
		Code:    errorAttr.Code,
		Msgcode: errorAttr.MsgCode,
		Field:   fieldName,
	}
}

// AddVals is a method that attaches values to a WscValidationError.
// It can be used to add extra information to the WscValidationError.
func (e *WscValidationError) AddVals(vals []string) {
	e.Vals = append(e.Vals, vals...)
}
