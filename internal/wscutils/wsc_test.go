package wscutils

import (
	"reflect"
	"testing"
)

// Test data structure
type TestUser struct {
	Fullname string `validate:"required"`
	Email    string `validate:"required,email"`
	Age      int    `validate:"min=10,max=150"`
}

func TestWscValidate(t *testing.T) {
	// Arrange
	testData := TestUser{
		Fullname: "",
		Email:    "invalid email",
		Age:      5,
	}

	expectedErrors := []ErrorMessage{
		BuildValidationError("Fullname", "required"),
		BuildValidationError("Email", "email"),
		BuildValidationError("Age", "min"),
	}

	// Act
	resultErrors := WscValidate(testData)

	// Assert
	if len(resultErrors) != len(expectedErrors) {
		t.Errorf("expected %v errors but got %v", len(expectedErrors), len(resultErrors))
	}

	for i, v := range expectedErrors {
		if !reflect.DeepEqual(v, resultErrors[i]) {
			t.Errorf("expected error %v but got %v", v, resultErrors[i])
		}
	}
}
