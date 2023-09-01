package main

import (
	"encoding/json"
	"io"
	"os"
)

type ErrorAttrSet struct {
	Code    string `json:"code"`
	MsgCode int    `json:"msgcode"`
}

// ErrorTypes is a map from error type to its corresponding attributes.
var ErrorTypes map[string]ErrorAttrSet

func init() {
	ErrorTypes = make(map[string]ErrorAttrSet)
	loadErrorTypes()
}

// loadErrorTypes loads the defined error types and their attributes into
// the ErrorTypes map from a JSON/YAML file.
// Eventually ErrorTypes will be loaded from the outside -- maybe the config package
func loadErrorTypes() {
	jsonFile, err := os.Open("errortypes.yaml")
	if err != nil {
		panic(err)
	}
	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)

	err = json.Unmarshal(byteValue, &ErrorTypes)
	if err != nil {
		panic(err)
	}
}

// GetValidationError creates a ValidationError for the given field name and error type.
// The values are loaded from ErrorTypes.
func GetValidationError(fieldName, errorType string, vals []string) ValidationError {
	errorAttr, exists := ErrorTypes[errorType]
	if !exists {
		errorAttr = ErrorTypes["unknown"]
	}

	return ValidationError{
		Code:    errorAttr.Code,
		Msgcode: errorAttr.MsgCode,
		Field:   fieldName,
		Vals:    vals,
	}
}
