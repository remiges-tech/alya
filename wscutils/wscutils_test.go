package wscutils

import (
	"os"
	"strconv"
	"strings"
	"testing"

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
