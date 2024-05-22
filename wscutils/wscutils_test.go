package wscutils

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func setup() {
	// Set default message ID and error code for validation errors
	SetDefaultMsgID(9999)
	SetDefaultErrCode("default_error")

	// Set a custom message ID for invalid JSON errors
	SetMsgIDInvalidJSON(1001)
	SetErrCodeInvalidJSON("invalid_json")

	// Register any other necessary mappings for validation tags to message IDs and error codes
	customValidationMap := map[string]int{
		"required": 1001,
		"email":    1002,
		"min":      1003,
		"max":      1004,
	}
	SetValidationTagToMsgIDMap(customValidationMap)

	customErrCodeMap := map[string]string{
		"required": "required",
		"email":    "email",
		"min":      "min",
		"max":      "max",
	}
	SetValidationTagToErrCodeMap(customErrCodeMap)
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	os.Exit(code)
}

type TestUser struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
	Age   int    `validate:"min=18,max=150"`
}

func TestSendSuccessResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	test := struct {
		name     string
		response *Response
		expected string
	}{
		name:     "Success response",
		response: NewSuccessResponse("test data"),
		expected: `{"status":"success","data":"test data","messages":null}`,
	}

	t.Run(test.name, func(t *testing.T) {
		// Create a response recorder
		w := httptest.NewRecorder()

		// Create a gin context with the response recorder as the writer
		c, _ := gin.CreateTestContext(w)

		// Call the function with a test response
		SendSuccessResponse(c, test.response)

		// Assert that the response body was correctly set
		assert.Equal(t, test.expected, w.Body.String())
	})
}

func TestSendErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Assuming ErrMsgIDInvalidJson is defined and represents the message ID for invalid JSON errors.
	msgID := msgIDInvalidJSON

	test := struct {
		name     string
		response *Response
		expected string
	}{
		name: "Error response",
		// Updated to include the msgID parameter.
		response: NewErrorResponse(msgID, ErrcodeInvalidJson),
		expected: `{"status":"error","data":null,"messages":[{"msgid":` + strconv.Itoa(msgID) + `,"errcode":"invalid_json"}]}`,
	}

	t.Run(test.name, func(t *testing.T) {
		// Create a response recorder
		w := httptest.NewRecorder()

		// Create a gin context with the response recorder as the writer
		c, _ := gin.CreateTestContext(w)

		// Call the function with a test response
		SendErrorResponse(c, test.response)

		// Assert that the response body was correctly set
		assert.Equal(t, test.expected, w.Body.String())
	})
}

// Adjusted getVals to return multiple values for a hypothetical "MultiValField".
func getVals(err validator.FieldError) []string {
	if err.Field() == "Age" {
		return []string{"10", "18-65"}
	}
	return []string{err.Field()}
}

const DefaultMsgID = 9999

func TestWscValidate(t *testing.T) {
	// Define test cases
	tests := []struct {
		name    string
		input   TestUser
		wantErr bool
		errMsgs []ErrorMessage // Expected error messages
	}{
		{
			name:    "Valid input",
			input:   TestUser{Name: "John Doe", Email: "john@example.com", Age: 18},
			wantErr: false,
			errMsgs: nil,
		},
		{
			name:    "Missing name",
			input:   TestUser{Email: "john@example.com", Age: 20},
			wantErr: true,
			errMsgs: []ErrorMessage{{MsgID: 1001, ErrCode: "required", Field: "Name", Vals: []string{"Name"}}},
		},
		{
			name:    "Invalid email",
			input:   TestUser{Name: "John Doe", Email: "not-an-email", Age: 20},
			wantErr: true,
			errMsgs: []ErrorMessage{{MsgID: 1002, ErrCode: "email", Field: "Email", Vals: []string{"Email"}}},
		},
		{
			name:    "Field with multiple values",
			input:   TestUser{Name: "MultiValField", Email: "john@example.com", Age: 10},
			wantErr: true,
			errMsgs: []ErrorMessage{
				{
					MsgID:   1003,
					ErrCode: "min",
					Field:   "Age",
					Vals:    []string{"10", "18-65"},
				},
			},
		},
	}

	// Iterate over test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsgs := WscValidate(tt.input, getVals)

			if (len(errMsgs) > 0) != tt.wantErr {
				t.Errorf("WscValidate() error = %v, wantErr %v", len(errMsgs) > 0, tt.wantErr)
			}

			if !reflect.DeepEqual(errMsgs, tt.errMsgs) {
				t.Errorf("WscValidate() got %v, want %v", errMsgs, tt.errMsgs)
			}
		})
	}
}

// Helper function to get a pointer to a string (for Field in ErrorMessage).
func pointerToString(s string) *string {
	return &s
}

func TestBindJSON_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Define a struct that matches the expected data structure.
	type TestData struct {
		Name string `json:"name"`
	}

	// Define test cases
	tests := []struct {
		name    string
		jsonStr string
		want    TestData
	}{
		{
			name:    "Single field",
			jsonStr: `{"data": {"name": "John Doe"}}`,
			want:    TestData{Name: "John Doe"},
		},
		{
			name:    "Empty name",
			jsonStr: `{"data": {"name": ""}}`,
			want:    TestData{Name: ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate a JSON request body
			body := bytes.NewBufferString(tc.jsonStr)
			req, _ := http.NewRequest(http.MethodPost, "/", body)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// The data variable where the JSON will be bound.
			var data TestData

			// Call BindJSON with the context and the data variable.
			err := BindJSON(c, &data)

			// Assert that there is no error and the data is correctly bound.
			assert.Nil(t, err)
			assert.Equal(t, tc.want, data)
		})
	}
}

func TestBindJSON_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Define test cases
	tests := []struct {
		name         string
		jsonStr      string
		expectedCode int
		expectedBody string
	}{
		{
			name:         "Incorrect Structure",
			jsonStr:      `{"data": "invalid JSON"}`,
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"status":"error","data":null,"messages":[{"msgid":1001,"errcode":"invalid_json"}]}`,
		},
		{
			name:         "Malformed JSON",
			jsonStr:      `{"data": }`,
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"status":"error","data":null,"messages":[{"msgid":1001,"errcode":"invalid_json"}]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate a JSON request body
			body := bytes.NewBufferString(tc.jsonStr)
			req, _ := http.NewRequest(http.MethodPost, "/", body)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Attempt to bind JSON to a Request struct
			_ = BindJSON(c, &Request{})

			// Assert that the response code and body are as expected
			assert.Equal(t, tc.expectedCode, w.Code)
			assert.JSONEq(t, tc.expectedBody, w.Body.String())
		})
	}
}
