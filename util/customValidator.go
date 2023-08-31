package util

import (
	"regexp"
)

func ValidatorTest(str string) string {
	return "Hello " + str
}

func IsValidIndiaZip(val string) bool {
	return regexp.MustCompile(`^[1-9][0-9]{5}$`).MatchString(val)
}
