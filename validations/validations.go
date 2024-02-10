// Package validations provide validation functions
package validations

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ttacon/libphonenumber"
)

var (
	// Compile regex pattern for India Zip validation
	regexIndiaZip = regexp.MustCompile(`^[1-9][0-9]{5}$`)

	// Compile regex pattern for Aadhaar number validation
	regexAadhaarNumber = regexp.MustCompile(`^[1-9]{4}[ -]?[0-9]{4}[ -]?[0-9]{4}$`)

	// Compile regex pattern for PAN number validation
	regexPanNumber = regexp.MustCompile(`[A-Z]{5}[0-9]{4}[A-Z]{1}`)
)

// Default country code is set as IN
const DEFAULT_COUNTRY_CODE = "IN"
const DEFAULT_COUNTRY_CODE_3C = "IND"

// constant value of 1 for Mobile types in googllibphone library
const NUMBER_TYPE_MOBILE = 1

// constant value of 0 for Fixed line types in googllibphone library
const NUMBER_TYPE_FIXED_LINE = 0

// Variable to hold allowed set of file extensions
var FILE_EXT = []string{"doc", "docx", "png"}

const DATE_FORMAT = "2006-01-02"

// IsValidIndiaZip checks if a given string is a valid India zip code.
// val: the string to be validated as a zip code.
// returns: a boolean indicating whether the string is a valid India zip code.
func IsValidIndiaZip(val string) bool {
	return regexIndiaZip.MatchString(val)
}

// IsValidCountryCode2 checks if the provided country code is valid and is of 2 Character.
// val: the country code to validate.
// returns: a boolean indicating whether the string is a valid country code of 2 character.
func IsValidCountryCode2(val string) bool {
	//return (DEFAULT_COUNTRY.Alpha2() == strings.ToUpper(val))
	return DEFAULT_COUNTRY_CODE == strings.ToUpper(val)

}

// IsValidCountryCode3 checks if the provided country code is valid and is of 3 Character.
// val: the country code to validate.
// returns: a boolean indicating whether the string is a valid country code of 3 character.
func IsValidCountryCode3(val string) bool {
	//return (DEFAULT_COUNTRY.Alpha3() == strings.ToUpper(val))
	return DEFAULT_COUNTRY_CODE_3C == strings.ToUpper(val)

}

// IsValidFileType checks if the given value is a valid file type based on a list of allowed extensions.
// val: the string value representing the file name.
// allowedExts: a slice of strings representing allowed file extensions.
// returns: a boolean indicating whether the file type is valid.
func IsValidFileType(val string, allowedExts []string) bool {
	for _, ext := range allowedExts {
		if strings.HasSuffix(strings.ToLower(val), "."+strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

// IsValidAadhaarNumber checks if a given string is a valid Aadhaar number.
// val: The string to be checked.
// returns: a boolean indicating whether the given aadhaar number is valid.
func IsValidAadhaarNumber(val string) bool {
	return regexAadhaarNumber.MatchString(val)
}

// IsValidPanNumber checks if the given value is a valid PAN number.
// val: a string representing the PAN number.
// returns: a boolean indicating whether the given PAN number is valid.
func IsValidPanNumber(val string) bool {
	return regexPanNumber.MatchString(val)
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

// Write a function to validate the date of birth based on maximum age and minimum age passesed in the function
func IsValidDateOfBirth(val string, minAge float64, maxAge float64) bool {
	// convert val in date format
	date, err := time.Parse(DATE_FORMAT, val)
	if err == nil {
		age := time.Since(date).Hours() / 24 / 365
		if age >= minAge && age <= maxAge {
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}
