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

// CalculateAge calculates the precise age in years from a given birthdate to the current date,
// accurately accounting for leap years and the exact number of days in each month.
//
// Parameters:
// - birthDate: The birthdate as a time.Time object.
//
// Returns: The age in years as an integer.
func CalculateAge(birthDate time.Time) int {
	now := time.Now().UTC()
	years := now.Year() - birthDate.Year()

	// After subtracting the years, if the current date is before the birthdate this year, subtract one year.
	beforeBirthdayThisYear := now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day())
	if beforeBirthdayThisYear {
		years--
	}

	return years
}

const DATE_INPUT_FORMAT = "2006-01-02" // used for time.Parse()

// IsValidDateOfBirth validates the date of birth based on optional maximum and minimum age constraints,
// ensuring consistency across different time zones by using UTC for all calculations.
// This function leverages CalculateAge for precise age calculation.
//
// Parameters:
// - yyyymmddVal: the date of birth in string format, expected to be in "YYYY-MM-DD" format.
// - minAge: pointer to an int representing the minimum age constraint, or nil if no minimum age constraint is applied.
// - maxAge: pointer to an int representing the maximum age constraint, or nil if no maximum age constraint is applied.
//
// Returns: a boolean indicating whether the date of birth satisfies the specified age constraints.
func IsValidDateOfBirth(yyyymmddVal string, minAge, maxAge *int) bool {
	birthDate, err := time.ParseInLocation(DATE_INPUT_FORMAT, yyyymmddVal, time.UTC)
	if err != nil {
		return false
	}

	age := CalculateAge(birthDate)

	if minAge != nil && age < *minAge {
		return false
	}
	if maxAge != nil && age > *maxAge {
		return false
	}
	return true
}
