package wscutils

import (
	"errors"
	"net/http"

	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const MSG_CODE_VALIDATION_FAILED = "invalid"

var validationTagToMsgID map[string]int
var validationTagToErrCode map[string]string

// SetValidationTagToMsgIDMap updates the internal mapping of validation tags to message IDs.
func SetValidationTagToMsgIDMap(customMap map[string]int) {
	validationTagToMsgID = customMap
}

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
	Field   *string  `json:"field,omitempty"` // make it a pointer so it can be omitted
	Vals    []string `json:"vals,omitempty"`  // omit if Vals is empty
}

// WscValidate is a generic function that accepts any data structure,
// validates it according to struct tag-provided validation rules
// and returns a slice of ErrorMessage in case of validation errors.
// This function will not add `vals` that's required as per the specifications
// because it does not know the request-specific values.
// `vals` will be added to ErrorMessage by the caller.
// func WscValidate[T any](data T, getVals func(err validator.FieldError) []string) []ErrorMessage {
// 	var validationErrors []ErrorMessage

// 	validate := validator.New()

//		err := validate.Struct(data)
//		if err != nil {
//			var validationErrs validator.ValidationErrors
//			if errors.As(err, &validationErrs) {
//				for _, err := range validationErrs {
//					vals := getVals(err)
//					field := err.Field()
//					msgid, exists := validationTagToMsgID[err.Tag()]
//					if !exists {
//						msgid = DefaultMsgID // Assume DefaultMsgID is defined somewhere
//					}
//					// Use MSG_CODE_VALIDATION_FAILED as the default error code
//					vErr := BuildErrorMessage(msgid, MSG_CODE_VALIDATION_FAILED, &field, vals...)
//					validationErrors = append(validationErrors, vErr)
//				}
//			}
//		}
//		return validationErrors
//	}
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
				vErr := BuildErrorMessage(msgid, errcode, &field, vals...)
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
//
// TODO:
// NEW COMMENT:
// BuildErrorMessage generates an ErrorMessage which includes
// the required validation error information such as code, msgid, and optionally field names and values.
func BuildErrorMessage(msgid int, errcode string, fieldName *string, vals ...string) ErrorMessage {
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
// BindJson provides a standard way of binding incoming JSON data to a
// given request data structure. It incorporates error handling.
func BindJSON(c *gin.Context, data any) error {
	req := Request{Data: data}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Example msgid for ErrcodeInvalidJson is 1001. Replace 1001 with the actual msgid you intend to use.
		invalidJsonError := BuildErrorMessage(ErrMsgIDInvalidJson, ErrcodeInvalidJson, nil)
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
	errorMessage := BuildErrorMessage(msgid, errcode, nil) // Assuming `nil` is acceptable for the field name in this context.
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

// SetValidationTagToErrCodeMap updates the internal mapping of validation tags to error codes.
func SetValidationTagToErrCodeMap(customMap map[string]string) {
	validationTagToErrCode = customMap
}

// DefaultErrCode holds the default error code for validation errors.
// Its value can be set using the SetDefaultErrCode function.
var DefaultErrCode string

// SetDefaultErrCode allows external code to set a custom default error code.
func SetDefaultErrCode(errCode string) {
	DefaultErrCode = errCode
}

// defaultMsgID holds the default message ID for validation errors.
// Its value can be set using the SetDefaultMsgID function.
var defaultMsgID int

// SetDefaultMsgID allows external code to set a custom default message ID.
func SetDefaultMsgID(msgID int) {
	defaultMsgID = msgID
}
