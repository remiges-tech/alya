package main

import (
	"fmt"

	valid "go-framework/util"
)

func main() {
	fmt.Println("in main")
	fmt.Println(valid.CleanerTest("clean file"))
	fmt.Println(valid.ValidatorTest("validator file"))

	fmt.Println("\nValidate India Zip ")
	fmt.Println("Valid Zipcode 457845: ", valid.IsValidIndiaZip("457845"))
	fmt.Println("Invalid Zip code 057845: ", valid.IsValidIndiaZip("057845"))

	fmt.Println("\nValidate Country code")
	fmt.Println("Valid CC2 in: ", valid.IsValidCountryCode2("in"))
	fmt.Println("Valid CC2 IN: ", valid.IsValidCountryCode2("IN"))
	fmt.Println("Invalid CC2 US: ", valid.IsValidCountryCode2("US"))

	fmt.Println("Valid CC3 ind: ", valid.IsValidCountryCode3("ind"))
	fmt.Println("Valid CC3 IND: ", valid.IsValidCountryCode3("IND"))
	fmt.Println("Invalid CC3 IIN: ", valid.IsValidCountryCode3("IIN"))

	fmt.Println("\nValidate File type")
	fmt.Println("File: ", valid.IsValidFileType("text.docx"))

	fmt.Println("\nValidate Aadhaar number")
	fmt.Println("Valid Aadhaar 1234 5678 9123: ", valid.IsValidAadhaarNumber("1234 5678 9123"))
	fmt.Println("Valid Aadhaar 123456789123: ", valid.IsValidAadhaarNumber("123456789123"))
	fmt.Println("Valid Aadhaar 1234-5678-9123: ", valid.IsValidAadhaarNumber("1234-5678-9123"))

	fmt.Println("Invalid Aadhaar 0234 5678 9123: ", valid.IsValidAadhaarNumber("0234 5678 9123"))
	fmt.Println("Invalid Aadhaar 023456789123: ", valid.IsValidAadhaarNumber("023456789123"))
	fmt.Println("Invalid Aadhaar 0234-5678-9123: ", valid.IsValidAadhaarNumber("0234-5678-9123"))

}
