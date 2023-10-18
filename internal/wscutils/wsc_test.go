package wscutils

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

var (
	testData = TestUser{
		Email: "invalid email",
		Age:   5,
	}
)

func TestMain(m *testing.M) {
	// Setup
	testErrorTypes := `
    required:
        code: "missing"
        msgcode: 45
    email:
        code: "invalid_email"
        msgcode: 50
    min:
        code: "outofrange"
        msgcode: 55
    `
	errorTypesReader := strings.NewReader(testErrorTypes)
	loadErrorTypes(errorTypesReader)

	// Run tests
	code := m.Run()

	// Teardown if necessary

	os.Exit(code)
}

// Test data structure
type TestUser struct {
	Fullname string `validate:"required"`
	Email    string `validate:"required,email"`
	Age      int    `validate:"min=10,max=150"`
}

func TestWscValidate(t *testing.T) {
	expectedErrors := []ErrorMessage{
		BuildErrorMessage("Fullname", "required"),
		BuildErrorMessage("Email", "email"),
		BuildErrorMessage("Age", "outofrange", "5"),
	}

	resultErrors := WscValidate(testData, func(err validator.FieldError) []string { return []string{strconv.Itoa(testData.Age)} })

	// Assert
	if len(resultErrors) != len(expectedErrors) {
		t.Errorf("expected %v errors but got %v", len(expectedErrors), len(resultErrors))
	}

	for i, v := range expectedErrors {
		if v.Field != resultErrors[i].Field || v.Code != resultErrors[i].Code {
			t.Errorf("expected error %v but got %v", v, resultErrors[i])
		}
	}
}

func TestAgeValsPopulation(t *testing.T) {
	getVals := func(err validator.FieldError) []string {
		// Only add the 'Age' related value just to keep it simple
		if err.Field() == "Age" {
			return []string{strconv.Itoa(testData.Age)}
		}
		return []string{}
	}

	resultErrors := WscValidate(testData, getVals)

	minAgeErrorPresent := false
	for _, err := range resultErrors {
		if err.Field == "Age" && err.Code == "outofrange" {
			// Check if the Vals array contains the actual age
			for _, val := range err.Vals {
				if val == strconv.Itoa(testData.Age) {
					minAgeErrorPresent = true
					break
				}
			}
		}
	}

	if !minAgeErrorPresent {
		t.Error("expected 'min' error for 'Age' field with value 5, but it was not present")
	}
}

func TestBuildValidationError(t *testing.T) {
	// Arrange
	field := "Age"
	errType := "min"

	// Load error types
	testErrorTypes := `
    required:
        code: "missing"
        msgcode: 45
    email:
        code: "invalid_email"
        msgcode: 50
    min:
        code: "outofrange"
        msgcode: 55
    `
	errorTypesReader := strings.NewReader(testErrorTypes)
	loadErrorTypes(errorTypesReader)

	// Act
	resultError := BuildErrorMessage(field, errType)

	// Assert
	if resultError.Field != "Age" {
		t.Errorf("expected field 'Age' but got %v", resultError.Field)
	}

	if resultError.Code != "outofrange" {
		t.Errorf("expected error code 'outofrange' but got %v", resultError.Code)
	}

	if resultError.Msgcode != 55 {
		t.Errorf("expected msgcode 55 but got %v", resultError.Msgcode)
	}

	if len(resultError.Vals) != 0 {
		t.Errorf("expected no values but got %v", resultError.Vals)
	}
}
