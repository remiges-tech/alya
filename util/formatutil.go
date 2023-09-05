// This package convert the numbers into words.
package formatutil

import (
	"errors"
	"strconv"

	"github.com/divan/num2words"
)

// This is ConvertNums2Words function. IT used to conveer the numbers into words.
// It uses num2words package to convert Numbers to words
// It takes numbers as a input parameters.
// It returns strings in return types.
func ConvertNums2Words(input_num string) (string, error) {

	// If no input_num was given, return an error with a message.
	if input_num == "" {
		return input_num, errors.New("empty string")
	}
	// Create a response using a random format.
	num, err := strconv.Atoi(input_num)
	if err != nil {
		return input_num, errors.New("Not a valid integer")
	}
	// if its valid number then it will return proper string
	return num2words.ConvertAnd(num), nil
}
