// Package validations provide validation functions
package validations

import (
	"regexp"
	"strings"
	"time"

	"github.com/nyaruka/phonenumbers"
)

// Variable to hold allowed set of file extensions
var FILE_EXT = []string{"doc", "docx", "png"}

// Compile regex pattern for India Zip validation
var regexIndiaZip = regexp.MustCompile(`^[1-9][0-9]{5}$`)

// IsValidIndiaZip checks if a given string is a valid India zip code.
// val: the string to be validated as a zip code.
// returns: a boolean indicating whether the string is a valid India zip code.
func IsValidIndiaZip(val string) bool {
	return regexIndiaZip.MatchString(val)
}

// IsValidFileType checks if the given value is a valid file type based on a list of allowed extensions.
// val: the string value representing the file name.
// allowedExts: a slice of strings representing allowed file extensions.
// returns: a boolean indicating whether the file type is valid.
func IsFileTypeAllowed(val string, allowedExts []string) bool {
	for _, ext := range allowedExts {
		if strings.HasSuffix(strings.ToLower(val), "."+strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

// Compile regex pattern for Aadhaar number validation
var regexAadhaarNumber = regexp.MustCompile(`^[1-9]{4}[ -]?[0-9]{4}[ -]?[0-9]{4}$`)

// IsValidAadhaarNumber checks if a given string is a valid Aadhaar number.
// val: The string to be checked.
// returns: a boolean indicating whether the given aadhaar number is valid.
func IsValidAadhaarNumber(val string) bool {
	return regexAadhaarNumber.MatchString(val)
}

// Compile regex pattern for PAN number validation
var regexPanNumber = regexp.MustCompile(`[A-Z]{5}[0-9]{4}[A-Z]{1}`)

// IsValidPanNumber checks if the given value is a valid PAN number.
// val: a string representing the PAN number.
// returns: a boolean indicating whether the given PAN number is valid.
func IsValidPanNumber(val string) bool {
	return regexPanNumber.MatchString(val)
}

// IsValidPhoneNumber checks if a phone number is considered valid for a specific region.
// regionCode must be a two-character uppercase ISO 3166-1 alpha-2 country code (e.g., "US" for United States, "IN" for India, "GB" for United Kingdom).
func IsValidPhoneNumber(phoneNumber string, regionCode string) bool {
	num, err := phonenumbers.Parse(phoneNumber, regionCode)
	if err != nil {
		return false
	}
	return phonenumbers.IsValidNumber(num)
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
