package wscutils

import (
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

// Request represents the standard structure of a request to the web service.
type Request struct {
	Ver  int `json:"ver" binding:"required"`
	Data any `json:"data" binding:"required"`
}

// Response represents the standard structure of a response of the web service.
type Response struct {
	Status   string         `json:"status"`
	Data     any            `json:"data"`
	Messages []ErrorMessage `json:"messages"`
}

// ErrorMessage defines the format of error part of the standard response object
// See: https://redmine.bquanta.xyz/projects/mail-doc/wiki/Websvcgeneral#Web-service-response-format
type ErrorMessage struct {
	Field   string   `json:"field"`
	Code    string   `json:"code"`
	Msgcode int      `json:"msgcode"`
	Vals    []string `json:"vals,omitempty"` // omit if Vals is empty
}

// errorTypes is a map from error type to its corresponding attributes.
var errorTypes map[string]ErrorMessage

// loadErrorTypes is used for loading predefined error types
func loadErrorTypes(errorTypesReader io.Reader) {
	byteValue, _ := io.ReadAll(errorTypesReader)

	err := yaml.Unmarshal(byteValue, &errorTypes)
	if err != nil {
		log.Panic(err)
	}
}

// WscValidate is a generic function that accepts any data structure,
// validates it according to struct tag-provided validation rules
// and returns a slice of ErrorMessage in case of validation errors.
// This function will not add `vals` that's required as per the specifications
// because it does not know the request-specific values.
// `vals` will be added to ErrorMessage by the caller.
func WscValidate[T any](data T, getVals func(err validator.FieldError) []string) []ErrorMessage {
	var validationErrors []ErrorMessage

	validate := validator.New()

	err := validate.Struct(data)

	if err != nil {
		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			for _, err := range validationErrs {
				// We handle validation error creation for developers.
				vals := getVals(err)
				vErr := BuildErrorMessage(err.Field(), err.Tag(), vals...)
				validationErrors = append(validationErrors, vErr)
			}
		}
	}
	return validationErrors
}

// BuildErrorMessage generates a ErrorMessage which includes
// the required validation error information such as code, msgcode
// It encapsulates the process of building an error message for consistency.
// Examples:
// Without vals
// errorMessage := BuildErrorMessage("field1", "error1")
//
// With vals
// errorMessage := BuildErrorMessage("field2", "error2", "val1", "val2")
func BuildErrorMessage(fieldName, errorType string, vals ...string) ErrorMessage {
	errorMessage, exists := errorTypes[errorType]

	// if the provided errorType doesn't exist, use attributes of "unknown" error type
	// and set the code (string) to the provided errorType for debugging
	if !exists {
		log.Printf("Unrecognized error type: %s", errorType)
		errorMessage = ErrorMessage{
			Field: fieldName,
			Code:  errorType,
		}
	}

	errorMessage.Field = fieldName
	errorMessage.Vals = vals
	return errorMessage
}

// NewResponse is a helper function to create a new web service response
// and any error messages that might need to be sent back to the client. It allows
// for a consistent structure in all API responses
func NewResponse(status string, data any, messages []ErrorMessage) *Response {
	return &Response{
		Status:   status,
		Data:     data,
		Messages: messages,
	}
}

// BindJson provides a standard way of binding incoming JSON data to a
// given request data structure. It incorporates error handling .
func BindJson(c *gin.Context, data interface{}) error {
	req := Request{Data: data}
	if err := c.ShouldBindJSON(&req); err != nil {
		invalidJsonError := BuildErrorMessage("", InvalidJSON)
		c.JSON(http.StatusBadRequest, NewResponse(ErrorStatus, nil, []ErrorMessage{invalidJsonError}))
		return err
	}
	return nil
}

// NewErrorResponse simplifies the process of creating a standard error response
// with a single error message
func NewErrorResponse(field, errType string) *Response {
	return NewResponse(ErrorStatus, nil, []ErrorMessage{BuildErrorMessage(field, errType)})
}

// NewSuccessResponse simplifies the process of creating a standard success response
func NewSuccessResponse(data interface{}) *Response {
	return NewResponse(SuccessStatus, data, nil)
}
