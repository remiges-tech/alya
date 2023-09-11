// This package provide testing utilities for IsValidFileType function
// This covers the table-driven test and Exmample for the IsValidFileType function for documentation
package util_test

import (
	"fmt"

	valid "go-framework/util"
	"testing"
)

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
		if output := valid.IsValidFileType(val.filename); output != val.expected {
			t.Errorf("got %v, wanted %v", output, val.expected)
		}
	}
}

// ExampleIsValidFileType generates examples of valid and invalid file names and prints the results.
// No parameters.
// No return value.
func ExampleIsValidFileType() {
	fmt.Println("Valid file type examples")
	fmt.Println("text.doc: ", valid.IsValidFileType("text.doc"))
	fmt.Println("text.png: ", valid.IsValidFileType("text.png"))

	fmt.Println("Invalid file type examples")
	fmt.Println("text.txt: ", valid.IsValidFileType("text.txt"))
	fmt.Println("text: ", valid.IsValidFileType("text"))

	// Output:
	// Valid file type examples
	// text.doc:  true
	// text.png:  true
	// Invalid file type examples
	// text.txt:  false
	// text:  false
}
