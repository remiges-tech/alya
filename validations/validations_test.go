package validations

import (
	"fmt"
	"time"

	"testing"
)

// mobileNumber means argument 1 and the expected stands for the 'result we expect'

func TestIsValidMobileNumber(t *testing.T) {
	testCases := []struct {
		phoneNumber string
		regionCode  string
		isValid     bool
	}{
		{"+14155552671", "US", true},
		{"+1415-555-2671", "US", true},
		{"415-555-2671", "US", true},
		{"+919876543210", "IN", true},
		{"+91 9876 5432 10", "IN", true},
		{"+919999", "IN", false},
		{"9876543210", "US", false},
		{"abc123", "US", false},
		{"+44201234567", "GB", true},
	}

	for _, testCase := range testCases {
		isValid := IsValidPhoneNumber(testCase.phoneNumber, testCase.regionCode)
		if isValid == testCase.isValid {
			fmt.Println("Test passed:", testCase.phoneNumber)
		} else {
			fmt.Println("Test failed:", testCase.phoneNumber)
		}
	}
}

type testFileType struct {
	filename    string
	allowedExts []string
	expected    bool
}

var testFileTypes = []testFileType{
	{"text.doc", []string{"doc", "docx", "png"}, true},
	{"text.docx", []string{"doc", "docx", "png"}, true},
	{"text.txt", []string{"doc", "docx", "png"}, false},
	{"text.jpg", []string{"doc", "docx", "png"}, false},
	{"text.png", []string{"doc", "docx", "png"}, true},
	{"", []string{"doc", "docx", "png"}, false},
	{"text", []string{"doc", "docx", "png"}, false},
}

func TestIsValidFileType(t *testing.T) {
	for _, val := range testFileTypes {
		if output := IsValidFileType(val.filename, val.allowedExts); output != val.expected {
			t.Errorf("IsValidFileType(%q, %v) = %v, wanted %v", val.filename, val.allowedExts, output, val.expected)
		}
	}
}

func TestCalculateAge(t *testing.T) {
	// Assume the current year for the test is 2024
	assumedCurrentYear := 2024
	actualCurrentYear := time.Now().Year()
	yearDifference := actualCurrentYear - assumedCurrentYear

	// Define test cases
	var tests = []struct {
		name      string
		birthDate time.Time
		expected  int
	}{
		{"Age 30", time.Date(1990, 10, 10, 0, 0, 0, 0, time.UTC), 33},
		{"Born Today", time.Now().UTC(), 0},
		{"Leap Year", time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC), 23},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.expected += yearDifference // Adjust the expected age

			if got := CalculateAge(tt.birthDate); got != tt.expected {
				t.Errorf("CalculateAge(%v) = %v, want %v", tt.birthDate, got, tt.expected)
			}
		})
	}
}

func TestIsValidDateOfBirth(t *testing.T) {
	// Define test cases
	var tests = []struct {
		name     string
		val      string
		minAge   *int
		maxAge   *int
		expected bool
	}{
		{"Valid Age Within Range", "1990-10-10", intPointer(18), intPointer(40), true},
		{"Too Young", "2010-01-01", intPointer(18), intPointer(40), false},
		{"Too Old", "1950-01-01", intPointer(18), intPointer(40), false},
		{"Only Min Age", "2005-01-01", intPointer(18), nil, true},
		{"Only Max Age", "2005-01-01", nil, intPointer(15), false},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidDateOfBirth(tt.val, tt.minAge, tt.maxAge); got != tt.expected {
				t.Errorf("IsValidDateOfBirth(%v, %v, %v) = %v, want %v", tt.val, tt.minAge, tt.maxAge, got, tt.expected)
			}
		})
	}
}

// intPointer is a helper function to create pointers for int values in test cases.
func intPointer(value int) *int {
	return &value
}

const MIN_AGE = 18
const MAX_AGE = 65

// ExampleIsValidDateOfBirth generates examples of valid and invalid file names and prints the results.
// No parameters.
// No return value.
func ExampleIsValidDateOfBirth() {
	fmt.Println("Valid DOB examples")
	fmt.Println("1999-05-05: ", IsValidDateOfBirth("1999-05-05", intPointer(MIN_AGE), intPointer(MAX_AGE)))
	fmt.Println("2000-05-05: ", IsValidDateOfBirth("2000-05-05", intPointer(MIN_AGE), intPointer(MAX_AGE)))

	fmt.Println("Invalid DOB examples")
	fmt.Println("2012-05-05: ", IsValidDateOfBirth("2012-05-05", intPointer(MIN_AGE), intPointer(MAX_AGE)))
	fmt.Println("text: ", IsValidDateOfBirth("2006-09-02", intPointer(MIN_AGE), intPointer(MAX_AGE)))

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
	fmt.Println("text.doc: ", IsValidFileType("text.doc", []string{"doc", "docx", "png"}))
	fmt.Println("text.png: ", IsValidFileType("text.png", []string{"doc", "docx", "png"}))

	fmt.Println("Invalid file type examples")
	fmt.Println("text.txt: ", IsValidFileType("text.txt", []string{"doc", "docx", "png"}))
	fmt.Println("text: ", IsValidFileType("text", []string{"doc", "docx", "png"}))
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
	fmt.Println("Valid phone number examples")
	fmt.Println("+918888888888: ", IsValidPhoneNumber("+918888888888", "IN"))
	fmt.Println("8888888888: ", IsValidPhoneNumber("8888888888", "IN")) // Local format

	fmt.Println("Invalid phone number examples")
	fmt.Println("+9111111111111: ", IsValidPhoneNumber("+9111111111111", "IN"))
	fmt.Println("11111111111: ", IsValidPhoneNumber("11111111111", "IN"))
	fmt.Println("022274688879: ", IsValidPhoneNumber("022274688879", "IN")) // Incorrect landline format

	// Output:
	// Valid phone number examples
	// +918888888888:  true
	// 8888888888:  true
	// Invalid phone number examples
	// +9111111111111:  false
	// 11111111111:  false
	// 022274688879:  false
}
