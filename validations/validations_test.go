package validations

import (
	"fmt"

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
		if output := IsValidMobileNumber(val.mobileNumber); output != val.expected {
			t.Errorf("got %v, wanted %v", output, val.expected)
		}
	}
}

// filename means argument 1 and the expected stands for the 'result we expect'
type testFileType struct {
	filename string
	expected bool
}

// Struct object for Table-Driven test with various valid and invalid file names
var testFileTypes = []testFileType{
	{"text.doc", true},
	{"text.docx", true},
	{"text.txt", false},
	{"text.jpg", false},
	{"text.png", true},
	{"", false},
	{"text", false},
}

// Function for Table-Driven test
// TestIsValidFileType tests the IsValidFileType function.
// It iterates over the testFileTypes slice and checks if the output of
// IsValidFileType matches the expected value. If the output doesn't match,
// it reports an error using the t.Errorf function.
func TestIsValidFileType(t *testing.T) {
	for _, val := range testFileTypes {
		if output := IsValidFileType(val.filename); output != val.expected {
			t.Errorf("got %v, wanted %v", output, val.expected)
		}
	}
}

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
		if output := IsValidDateOfBirth(val.dob, MIN_AGE, MAX_AGE); output != val.expected {
			t.Errorf("got %v, wanted %v", output, val.expected)
		}
	}
}

// ExampleIsValidDateOfBirth generates examples of valid and invalid file names and prints the results.
// No parameters.
// No return value.
func ExampleIsValidDateOfBirth() {
	fmt.Println("Valid DOB examples")
	fmt.Println("1999-05-05: ", IsValidDateOfBirth("1999-05-05", MIN_AGE, MAX_AGE))
	fmt.Println("2000-05-05: ", IsValidDateOfBirth("2000-05-05", MIN_AGE, MAX_AGE))

	fmt.Println("Invalid DOB examples")
	fmt.Println("2012-05-05: ", IsValidDateOfBirth("2012-05-05", MIN_AGE, MAX_AGE))
	fmt.Println("text: ", IsValidDateOfBirth("2006-09-02", MIN_AGE, MAX_AGE))

	// Output:
	// Valid DOB examples
	// 1999-05-05:  true
	// 2000-05-05:  true
	// Invalid DOB examples
	// 2012-05-05:  false
	// text:  false
}

// ExampleIsValidFileType generates examples of valid and invalid file names and prints the results.
// No parameters.
// No return value.
func ExampleIsValidFileType() {
	fmt.Println("Valid file type examples")
	fmt.Println("text.doc: ", IsValidFileType("text.doc"))
	fmt.Println("text.png: ", IsValidFileType("text.png"))

	fmt.Println("Invalid file type examples")
	fmt.Println("text.txt: ", IsValidFileType("text.txt"))
	fmt.Println("text: ", IsValidFileType("text"))

	// Output:
	// Valid file type examples
	// text.doc:  true
	// text.png:  true
	// Invalid file type examples
	// text.txt:  false
	// text:  false
}

// ExampleIsValidMobileNumber generates examples of valid and invalid mobile numbers and prints the results.
// No parameters.
// No return value.
func ExampleIsValidMobileNumber() {
	fmt.Println("Valid mobile number examples")
	fmt.Println("+918888888888: ", IsValidMobileNumber("+918888888888"))
	fmt.Println("8888888888: ", IsValidMobileNumber("8888888888"))

	fmt.Println("Invalid mobile number examples")
	fmt.Println("+9111111111111: ", IsValidMobileNumber("+9111111111111"))
	fmt.Println("11111111111: ", IsValidMobileNumber("11111111111"))
	fmt.Println("022274688879: ", IsValidMobileNumber("022274688879"))

	// Output:
	// Valid mobile number examples
	// +918888888888:  true
	// 8888888888:  true
	// Invalid mobile number examples
	// +9111111111111:  false
	// 11111111111:  false
	// 022274688879:  false
}
