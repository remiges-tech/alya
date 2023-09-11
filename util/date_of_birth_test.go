// This package provide testing utilities for IsValidDateOfBirth function
// This covers the table-driven test and Exmample for the IsValidDateOfBirth function for documentation
package util_test

import (
	"fmt"

	valid "go-framework/util"
	"testing"
)

const MIN_AGE = 18
const MAX_AGE = 65

// dob means argument 1 and the expected stands for the 'result we expect'
type testDOB struct {
	dob      string
	expected bool
}

// Struct object for Table-Driven test with various valid and invalid DOBs
var testDOBs = []testDOB{
	{"1999-05-05", true},
	{"2000-05-05", true},
	{"2012-05-05", false},
	{"2006-05-05", false},
	{"2006-13-05", false},
}

// Function for Table-Driven test
// TestIsValidDateOfBirth tests the IsValidDateOfBirth function.
// It iterates over the testDOBs slice and checks if the output of
// IsValidDateOfBirth matches the expected value. If the output doesn't match,
// it reports an error using the t.Errorf function.
func TestIsValidDateOfBirth(t *testing.T) {
	for _, val := range testDOBs {
		if output := valid.IsValidDateOfBirth(val.dob, MIN_AGE, MAX_AGE); output != val.expected {
			t.Errorf("got %v, wanted %v", output, val.expected)
		}
	}
}

// ExampleIsValidDateOfBirth generates examples of valid and invalid file names and prints the results.
// No parameters.
// No return value.
func ExampleIsValidDateOfBirth() {
	fmt.Println("Valid DOB examples")
	fmt.Println("1999-05-05: ", valid.IsValidDateOfBirth("1999-05-05", MIN_AGE, MAX_AGE))
	fmt.Println("2000-05-05: ", valid.IsValidDateOfBirth("2000-05-05", MIN_AGE, MAX_AGE))

	fmt.Println("Invalid DOB examples")
	fmt.Println("2012-05-05: ", valid.IsValidDateOfBirth("2012-05-05", MIN_AGE, MAX_AGE))
	fmt.Println("text: ", valid.IsValidDateOfBirth("2006-09-02", MIN_AGE, MAX_AGE))

	// Output:
	// Valid DOB examples
	// 1999-05-05:  true
	// 2000-05-05:  true
	// Invalid DOB examples
	// 2012-05-05:  false
	// text:  false
}
