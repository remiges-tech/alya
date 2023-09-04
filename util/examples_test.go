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

func TestIsValidMobileNumber(t *testing.T) {

	got := valid.IsValidMobileNumber("+918888888888")
	want := true

	if got != want {
		t.Errorf("got %v, wanted %v", got, want)
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
