package util

import (
	"regexp"
	"strings"

	"github.com/biter777/countries"
)

const DEFAULT_COUNTRY = countries.India

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
