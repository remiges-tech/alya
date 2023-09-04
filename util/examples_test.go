package util_test

import (
	"fmt"

	valid "go-framework/util"
	"testing"
)

func ExampleValidatorTest() {

	fmt.Println(valid.ValidatorTest("indu"))
	// Output:
	// Hello indu

}

// mobileNumber means argument 1 and the expected stands for the 'result we expect'
type mobileNumberTest struct {
	mobileNumber string
	expected     bool
}

// Struct object for Table-Driven test with various valid and invalid mobile numbers
var mobileNumberTests = []mobileNumberTest{
	{"+918888888888", true},
	{"8888888888", true},
	{"+9111111111111", false},
	{"11111111111", false},
	{"022274688879", false},
}

// Function for Table-Driven test
func TestIsValidMobileNumber(t *testing.T) {

	for _, val := range mobileNumberTests {
		if output := valid.IsValidMobileNumber(val.mobileNumber); output != val.expected {
			t.Errorf("got %v, wanted %v", output, val.expected)
		}
	}
}

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
