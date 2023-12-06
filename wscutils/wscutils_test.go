package wscutils

import (
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/assert/v2"
	"github.com/go-playground/validator/v10"
)

type TestUser struct {
	Fullname string `validate:"required"`
	Email    string `validate:"required,email"`
	Age      int    `validate:"min=10,max=150"`
}

var testData = TestUser{
	Email: "invalid email",
	Age:   5,
}

func TestMain(m *testing.M) {
	// Setup
	testErrorTypes := `
    required: 45
    email: 50
    min: 55
    `
	errorTypesReader := strings.NewReader(testErrorTypes)
	LoadErrorTypes(errorTypesReader)

	// Run tests
	code := m.Run()

	// Teardown if necessary

	os.Exit(code)
}

func TestWscValidate(t *testing.T) {
	fieldName1 := "Fullname"
	fieldName2 := "Email"
	fieldName3 := "Age"
	expectedErrors := []ErrorMessage{
		BuildErrorMessage("required", &fieldName1),
		BuildErrorMessage("email", &fieldName2),
		BuildErrorMessage("min", &fieldName3, "5"),
	}

	resultErrors := WscValidate(testData, func(err validator.FieldError) []string { return []string{strconv.Itoa(testData.Age)} })

	// Assert
	if len(resultErrors) != len(expectedErrors) {
		t.Errorf("expected %v errors but got %v", len(expectedErrors), len(resultErrors))
	}

	for i, v := range expectedErrors {
		if *v.Field != *resultErrors[i].Field || v.ErrCode != resultErrors[i].ErrCode {
			t.Errorf("expected error %v but got %v", v, resultErrors[i])
		}
	}
}

func TestBuildErrorMessage(t *testing.T) {
	fieldName := "TestField"
	errCode := "required"
	vals := []string{"5"}

	// Test without vals
	msg := BuildErrorMessage(errCode, &fieldName)
	if msg.ErrCode != errCode || *msg.Field != fieldName || len(msg.Vals) != 0 {
		t.Errorf("BuildErrorMessage() = %v; want ErrCode=%v, Field=%v, Vals=[]", msg, errCode, fieldName)
	}

	// Test with vals
	msg = BuildErrorMessage(errCode, &fieldName, vals...)
	if msg.ErrCode != errCode || *msg.Field != fieldName || len(msg.Vals) != len(vals) || msg.Vals[0] != vals[0] {
		t.Errorf("BuildErrorMessage() = %v; want ErrCode=%v, Field=%v, Vals=%v", msg, errCode, fieldName, vals)
	}

	// Test with unrecognized errcode
	unrecognizedErrCode := "unrecognized"
	msg = BuildErrorMessage(unrecognizedErrCode, &fieldName)
	if msg.ErrCode != unrecognizedErrCode || *msg.Field != fieldName || len(msg.Vals) != 0 {
		t.Errorf("BuildErrorMessage() = %v; want ErrCode=%v, Field=%v, Vals=[]", msg, unrecognizedErrCode, fieldName)
	}
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

	test := struct {
		name     string
		response *Response
		expected string
	}{
		name:     "Error response",
		response: NewErrorResponse(ErrcodeInvalidJson),
		expected: `{"status":"error","data":null,"messages":[{"msgid":0,"errcode":"invalid_json"}]}`,
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
