// Package util provide validation function
package util

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/biter777/countries"
	"github.com/ttacon/libphonenumber"
)

const DEFAULT_COUNTRY = countries.India
const DEFAULT_COUNTRY_CODE = "IN"
const NUMBER_TYPE_MOBILE = 1
const NUMBER_TYPE_FIXED_LINE = 0

var FILE_EXT = []string{"doc", "docx", "png"}

// ValidatorTest is a dummy test function
func ValidatorTest(str string) string {
	return "Hello " + str
}

// IsValidIndiaZip checks if a given string is a valid India zip code.
// val: the string to be validated as a zip code.
// returns: a boolean indicating whether the string is a valid India zip code.
func IsValidIndiaZip(val string) bool {
	return regexp.MustCompile(`^[1-9][0-9]{5}$`).MatchString(val)
}

// IsValidCountryCode2 checks if the provided country code is valid and is of 2 Character.
// val: the country code to validate.
// returns: a boolean indicating whether the string is a valid country code of 2 character.
func IsValidCountryCode2(val string) bool {
	return (DEFAULT_COUNTRY.Alpha2() == strings.ToUpper(val))
}

// IsValidCountryCode3 checks if the provided country code is valid and is of 3 Character.
// val: the country code to validate.
// returns: a boolean indicating whether the string is a valid country code of 3 character.
func IsValidCountryCode3(val string) bool {
	return (DEFAULT_COUNTRY.Alpha3() == strings.ToUpper(val))
}

// IsValidFileType checks if the given value is a valid file type.
// val: the string value representing the file name.
// returns: a boolean indicating whether the file type is valid.
func IsValidFileType(val string) bool {
	for _, ext := range FILE_EXT {
		if strings.HasSuffix(val, ext) {
			return true
		}
	}
	return false
}

// IsValidAadhaarNumber checks if a given string is a valid Aadhaar number.
// val: The string to be checked.
// returns: a boolean indicating whether the given aadhaar number is valid.
func IsValidAadhaarNumber(val string) bool {
	return regexp.MustCompile(`^[1-9]{4}[ -]?[0-9]{4}[ -]?[0-9]{4}$`).MatchString(val)
}

// IsValidPanNumber checks if the given value is a valid PAN number.
// val: a string representing the PAN number.
// returns: a boolean indicating whether the given PAN number is valid.
func IsValidPanNumber(val string) bool {
	return regexp.MustCompile(`[A-Z]{5}[0-9]{4}[A-Z]{1}`).MatchString(val)
}

// IsValidMobileNumber checks if a given string is a valid mobile number.
// val: the string to be checked as a mobile number, which represents the mobile number to be validated.
// returns: a boolean value indicating whether the given number is valid.
func IsValidMobileNumber(val string) bool {
	num, err := libphonenumber.Parse(val, DEFAULT_COUNTRY_CODE)
	//fmt.Printf("%T\n", num)
	if err != nil {
		fmt.Println("Err:", err)
		return false
	}
	if libphonenumber.IsValidNumberForRegion(num, DEFAULT_COUNTRY_CODE) && libphonenumber.GetNumberType(num) == NUMBER_TYPE_MOBILE {
		return true
	}
	return false
}
