package main

import (
	"fmt"

	valid "go-framework/util"
)

func main() {
	fmt.Println("in main")
	fmt.Println(valid.CleanerTest("clean file"))
	fmt.Println(valid.ValidatorTest("validator file"))
	fmt.Println(valid.IsValidIndiaZip("457845"))

	fmt.Println("Valid CC2 in: ", valid.IsValidCountryCode2("in"))
	fmt.Println("Valid CC2 IN: ", valid.IsValidCountryCode2("IN"))
	fmt.Println("Invalid CC2 US: ", valid.IsValidCountryCode2("US"))

	fmt.Println("Valid CC3 ind: ", valid.IsValidCountryCode3("ind"))
	fmt.Println("Valid CC3 IND: ", valid.IsValidCountryCode3("IND"))
	fmt.Println("Invalid CC3 IIN: ", valid.IsValidCountryCode3("IIN"))

}
