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

func ValidatorTest(str string) string {
	return "Hello " + str
}

func IsValidIndiaZip(val string) bool {
	return regexp.MustCompile(`^[1-9][0-9]{5}$`).MatchString(val)
}

func IsValidCountryCode2(val string) bool {
	return (DEFAULT_COUNTRY.Alpha2() == strings.ToUpper(val))
}

func IsValidCountryCode3(val string) bool {
	return (DEFAULT_COUNTRY.Alpha3() == strings.ToUpper(val))
}

func IsValidFileType(val string) bool {
	for _, ext := range FILE_EXT {
		if strings.HasSuffix(val, ext) {
			return true
		}
	}
	return false
}

func IsValidAadhaarNumber(val string) bool {
	return regexp.MustCompile(`^[1-9]{4}[ -]?[0-9]{4}[ -]?[0-9]{4}$`).MatchString(val)
}

// Write a function to validate GST number
func IsValidPanNumber(val string) bool {
	return regexp.MustCompile(`[A-Z]{5}[0-9]{4}[A-Z]{1}`).MatchString(val)
}

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
