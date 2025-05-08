package wscutils

import (
	"encoding/json"
	"errors"
	"net/http"

	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Request represents the standard structure of a request to the web service.
type Request struct {
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
	MsgID   int      `json:"msgid"`
	ErrCode string   `json:"errcode"`
	Field   string   `json:"field,omitempty"`
	Vals    []string `json:"vals,omitempty"`
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
				vals := getVals(err)
				field := err.Field()
				msgid, exists := validationTagToMsgID[err.Tag()]
				if !exists {
					msgid = defaultMsgID
				}
				errcode, codeExists := validationTagToErrCode[err.Tag()]
				if !codeExists {
					errcode = DefaultErrCode
				}
				vErr := BuildErrorMessage(msgid, errcode, field, vals...)
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
// errorMessage := BuildErrorMessage(1001, "retry", "field1", "error1")
//
// With vals
// errorMessage := BuildErrorMessage(1000, "invalid", "field2", "error2", "val1", "val2")
func BuildErrorMessage(msgid int, errcode string, fieldName string, vals ...string) ErrorMessage {
	errorMessage := ErrorMessage{
		MsgID:   msgid,
		ErrCode: errcode,
		Field:   fieldName,
		Vals:    vals,
	}

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
func BindJSON(c *gin.Context, data any) error {
	req := Request{Data: data}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Example msgid for ErrcodeInvalidJson is 1001. Replace 1001 with the actual msgid you intend to use.
		invalidJsonError := BuildErrorMessage(msgIDInvalidJSON, errCodeInvalidJSON, "")
		c.JSON(http.StatusBadRequest, NewResponse(ErrorStatus, nil, []ErrorMessage{invalidJsonError}))
		return err
	}
	return nil
}

// NewErrorResponse simplifies the process of creating a standard error response
// with a single error message. Now updated to include a msgid.
func NewErrorResponse(msgid int, errcode string) *Response {
	// Assuming `DefaultMsgID` is a suitable default message ID for general errors.
	// If `msgid` should be variable, pass it as a parameter to `NewErrorResponse`.
	errorMessage := BuildErrorMessage(msgid, errcode, "")
	return NewResponse(ErrorStatus, nil, []ErrorMessage{errorMessage})
}

// NewSuccessResponse simplifies the process of creating a standard success response
func NewSuccessResponse(data any) *Response {
	return NewResponse(SuccessStatus, data, nil)
}

// GetRequestUser extracts the requestUser from the gin context.
func GetRequestUser(c *gin.Context) (string, error) {
	requestUser, exists := c.Get("RequestUser")
	if !exists {
		return "", fmt.Errorf("missing_request_user")
	}

	requestUserStr, ok := requestUser.(string)
	if !ok {
		return "", fmt.Errorf("invalid_request_user")
	}

	return requestUserStr, nil
}

// SendSuccessResponse sends a JSON response.
func SendSuccessResponse(c *gin.Context, response *Response) {
	c.JSON(http.StatusOK, response)
}

// SendErrorResponse sends a JSON error response.
func SendErrorResponse(c *gin.Context, response *Response) {
	c.JSON(http.StatusBadRequest, response)
}

// validation specific error codes and msg IDs

var validationTagToMsgID map[string]int
var validationTagToErrCode map[string]string
var msgIDInvalidJSON int
var errCodeInvalidJSON string

// SetValidationTagToMsgIDMap updates the internal mapping of validation tags to message IDs.
// This function allows for the customization of message IDs associated with specific validation
// errors.
// The customMap parameter should contain a mapping of validation tags (e.g., "required", "email")
// to their corresponding message IDs.
func SetValidationTagToMsgIDMap(customMap map[string]int) {
	validationTagToMsgID = customMap
}

// SetValidationTagToErrCodeMap updates the internal mapping of validation tags to error codes.
// Similar to SetValidationTagToMsgIDMap, this function customizes the error codes returned in
// the response for specific validation errors. The customMap parameter should contain a mapping
// of validation tags to their corresponding error codes.
func SetValidationTagToErrCodeMap(customMap map[string]string) {
	validationTagToErrCode = customMap
}

// DefaultErrCode holds the default error code for validation errors.
// Its value can be set using the SetDefaultErrCode function.
var DefaultErrCode string

// defaultMsgID holds the default message ID for validation errors.
// Its value can be set using the SetDefaultMsgID function.
var defaultMsgID int

// SetDefaultMsgID allows external code to set a custom default message ID for validation errors.
// This ID is used as a fallback when a specific validation error does not have a message ID
// registered via SetValidationTagToMsgIDMap.
func SetDefaultMsgID(msgID int) {
	defaultMsgID = msgID
}

// SetDefaultErrCode allows external code to set a custom default error code for validation errors.
// Similar to SetDefaultMsgID, this function sets a fallback error code to be used when a specific
// validation error does not have an error code registered.
func SetDefaultErrCode(errCode string) {
	DefaultErrCode = errCode
}

// SetMsgIDInvalidJSON allows external code to set a custom message ID for errors related to invalid JSON.
func SetMsgIDInvalidJSON(msgID int) {
	msgIDInvalidJSON = msgID
}

// SetErrCodeInvalidJSON allows external code to set a custom error code for errors related to invalid JSON.
func SetErrCodeInvalidJSON(errCode string) {
	errCodeInvalidJSON = errCode
}

// Optional is a generic type that can distinguish between non-existent JSON fields and null values.
// It can be used in struct fields where you need to know if a field was:
// 1. Present in the JSON and had a value (Present = true, Null = false)
// 2. Present in the JSON but was null (Present = true, Null = true)
// 3. Not present in the JSON at all (Present = false)
//
// Example usage:
//
//	// Custom settings type
//	type Settings struct {
//	    Theme       string `json:"theme"`
//	    Notifications bool  `json:"notifications"`
//	}
//
//	type User struct {
//	    ID        int              `json:"id"`
//	    Name      string           `json:"name"`
//	    Email     Optional[string] `json:"email"`
//	    IsActive  Optional[bool]   `json:"isActive"`
//	    Settings  Optional[Settings] `json:"settings"`
//	}
//
//	// After unmarshaling JSON
//	var user User
//	json.Unmarshal(data, &user)
//
//	// Using the Get() method for simple access to values
//	if email, ok := user.Email.Get(); ok {
//	    fmt.Println("Email is provided:", email)
//	} else {
//	    fmt.Println("Email is absent or null")
//	}
//
//	// Similarly for boolean and custom type values
//	if isActive, ok := user.IsActive.Get(); ok {
//	    fmt.Println("User active status provided:", isActive)
//	}
type Optional[T any] struct {
	Value   T
	Present bool
	Null    bool
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// This allows Optional to detect both missing fields and explicit nulls during JSON unmarshaling.
// When a field is omitted completely from JSON:
// - UnmarshalJSON is never called for that field
// - The field retains its zero values (Present=false, Null=false)
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	// Check for null value
	if string(data) == "null" {
		o.Present = true
		o.Null = true
		return nil
	}

	// Not null, try to unmarshal into Value
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	o.Value = value
	o.Present = true
	o.Null = false
	return nil
}

// Get returns the Value and true if the Optional has a defined value,
// or the zero value of T and false if it doesn't have a value or is null.
// This method simplifies extracting values from Optional fields without
// having to write complex conditionals combining Present and Null fields.
//
// Example usage:
//
//	if value, ok := optional.Get(); ok {
//	    // Use value here, it exists and is not null
//	} else {
//	    // Handle missing or null case
//	}
func (o Optional[T]) Get() (T, bool) {
	if o.Present && !o.Null {
		return o.Value, true
	}
	var zero T
	return zero, false
}

// ValidatorValue implements the ValidatorValuer interface for validator v10.
// This allows validator to correctly validate the underlying value when validating structs that contain Optional fields.
// If the Optional has a value (Present=true and Null=false), it returns the Value.
// Otherwise, it returns the zero value of type T.
func (o Optional[T]) ValidatorValue() any {
	if o.Present && !o.Null {
		return o.Value
	}
	var zero T
	return zero
}
