// This package provide testing utilities for IsValidMobileNumber function
// This covers the table-driven test and Exmample for the IsValidMobileNumber function for documentation
package util_test

import (
	"fmt"

	valid "go-framework/util"
	"testing"
)

// mobileNumber means argument 1 and the expected stands for the 'result we expect'
type testMobileNumber struct {
	mobileNumber string
	expected     bool
}

// Struct object for Table-Driven test with various valid and invalid mobile numbers
var testMobileNumbers = []testMobileNumber{
	{"+918888888888", true},
	{"8888888888", true},
	{"+9111111111111", false},
	{"11111111111", false},
	{"022274688879", false},
	{"", false},
	{"asdf", false},
}

// Function for Table-Driven test
// TestIsValidMobileNumber tests the IsValidMobileNumber function.
// It iterates over the mobileNumberTests slice and checks if the output of
// IsValidMobileNumber matches the expected value. If the output doesn't match,
// it reports an error using the t.Errorf function.
func TestIsValidMobileNumber(t *testing.T) {
	for _, val := range testMobileNumbers {
		if output := valid.IsValidMobileNumber(val.mobileNumber); output != val.expected {
			t.Errorf("got %v, wanted %v", output, val.expected)
		}
	}
}

// ExampleIsValidMobileNumber generates examples of valid and invalid mobile numbers and prints the results.
// No parameters.
// No return value.
func ExampleIsValidMobileNumber() {
	fmt.Println("Valid mobile number examples")
	fmt.Println("+918888888888: ", valid.IsValidMobileNumber("+918888888888"))
	fmt.Println("8888888888: ", valid.IsValidMobileNumber("8888888888"))

	fmt.Println("Invalid mobile number examples")
	fmt.Println("+9111111111111: ", valid.IsValidMobileNumber("+9111111111111"))
	fmt.Println("11111111111: ", valid.IsValidMobileNumber("11111111111"))
	fmt.Println("022274688879: ", valid.IsValidMobileNumber("022274688879"))

	// Output:
	// Valid mobile number examples
	// +918888888888:  true
	// 8888888888:  true
	// Invalid mobile number examples
	// +9111111111111:  false
	// 11111111111:  false
	// 022274688879:  false
}
