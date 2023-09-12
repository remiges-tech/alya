// This package convert the numbers into words.
package formatutil

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// strings at index 0 is not used, it is
// to make array indexing simple
var slcUnits = []string{"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
var slcTens = []string{"", "ten", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
var slcTeens = []string{"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}

// Indian Muilitpliers or Units
var indianMuliplier = []string{"", "thousand", "lakh", "crore", "arab", "kharab", "neel", "padma", "shankh", "mahashankh"}

// International  Muilitpliers or Units
var englishMegas = []string{"", "thousand", "million", "billion", "trillion", "quadrillion", "quintillion", "sextillion", "septillion", "octillion", "nonillion", "decillion", "undecillion", "duodecillion", "tredecillion", "quattuordecillion"}

// const variables
const (
	// constant value of 2 for Max Demimal allowed/considered
	MAX_DECIMAL_NUMBER_ALLOWED = 2
	// constant value of 0 for Min Demimal allowed/considered
	MIN_DECIMAL_NUMBER_ALLOWED = 0

	//constant value of 1000 for thousands
	MAX_MULTIPLIER_ALLOWED = 1000
	//constant value of 100 for hundreds
	MIN_MULTIPLIER_ALLOWED = 100
	//constant value of 10
	MIN_TEN_MULTIPLIER = 10
)

// ConvertIntegerToEnIn converts an integer to its English representation in Indian format.
//
// Parameters:
// - input_num: the input number as a string.
//
// Returns:
// - the Indian English representation of the number, and the error is returned if the input_num is empty or not a valid number.
func ConvertIntegerToEnIn(input_num string) (string, error) {
	// If no input_num was given, return an error with a message.
	if input_num == "" {
		return input_num, errors.New("Empty string. Please provide valid number")
	}

	num, err := strconv.ParseFloat(input_num, 32)
	if err != nil {
		return input_num, errors.New("Not a valid number")
	}
	fmt.Println("Entered Number: ", num)
	slc := []string{}
	slc = strings.Split(input_num, ".")
	words := []string{}

	for i := 0; i < len(slc); i++ {
		number, err := strconv.Atoi(slc[i])
		if err != nil {
			return input_num, errors.New("Not a valid number")
		}
		// Create a response using a string, error format.

		// if its valid number then it will return proper string
		//approach 1 will accept 12 digit numbers
		if i == 0 {
			words = append(words, IntegerToEnIn(number))
			words = append(words, "Rupees")
		} else {
			if number >= MIN_MULTIPLIER_ALLOWED {
				temp_str := slc[i]
				//consider first two digits
				temp_str1 := temp_str[MIN_DECIMAL_NUMBER_ALLOWED:MAX_DECIMAL_NUMBER_ALLOWED]
				number, _ = strconv.Atoi(temp_str1)
			}
			words = append(words, "and "+IntegerToEnIn(number)+" Paise")
		}

	}
	return strings.Join(words, " "), nil

}

// ConvertIntegerToEnUS converts an input number to its English representation in US format.
//
// Parameters:
// - input_num: the input number as a string.
//
// Returns:
// - the Indian English representation of the number, and the error is returned if the input_num is empty or not a valid number.
func ConvertIntegerToEnUS(input_num string) (string, error) {
	// If no input_num was given, return an error with a message.
	if input_num == "" {
		return input_num, errors.New("Empty string. Please provide valid number")
	}

	num, err := strconv.ParseFloat(input_num, 32)
	if err != nil {
		return input_num, errors.New("Not a valid number")
	}
	fmt.Println("Entered Number: ", num)
	slc := []string{}
	slc = strings.Split(input_num, ".")
	words := []string{}

	for i := 0; i < len(slc); i++ {
		number, err := strconv.Atoi(slc[i])
		if err != nil {
			return input_num, errors.New("Not a valid number")
		}
		// Create a response using a string, error format.

		// if its valid number then it will return proper string
		//approach 1 will accept 12 digit numbers
		if i == 0 {
			words = append(words, IntegerToEnUs(number))
		} else {
			if number >= MIN_MULTIPLIER_ALLOWED {
				temp_str := slc[i]
				//consider first two digits
				temp_str1 := temp_str[MIN_DECIMAL_NUMBER_ALLOWED:MAX_DECIMAL_NUMBER_ALLOWED]
				number, _ = strconv.Atoi(temp_str1)
			}
			words = append(words, "and "+IntegerToEnUs(number))
		}

	}
	return strings.Join(words, " "), nil

}

// IntegerToEnIn converts an integer to indian words.
//
// Parameters:
// - input: the integer to convert.
//
// Returns:
// - the English representation of the input integer as a string.
func IntegerToEnIn(input int) string {
	//log.Printf("Input: %d\n", input)
	words := []string{}

	if input < 0 {
		words = append(words, "minus")
		input *= -1
	}

	// split integer in hybrids
	var hybrids []int
	hybrids = integerToDHybrid(input)
	// log.Printf("Hybrids: %v\n", hybrids)

	// zero is a special case
	if len(hybrids) == 0 {
		return "zero"
	}

	// iterate over hybrids
	for idx := len(hybrids) - 1; idx >= 0; idx-- {
		hybrid := hybrids[idx]
		//log.Printf("hybrid: %d (idx=%d)\n", hybrid, idx)

		// nothing todo for empty hybrid
		if hybrid == 0 {
			continue
		}

		// three-digits
		hundreds := hybrid / MIN_MULTIPLIER_ALLOWED % MIN_TEN_MULTIPLIER
		tens := hybrid / MIN_TEN_MULTIPLIER % MIN_TEN_MULTIPLIER
		units := hybrid % MIN_TEN_MULTIPLIER

		//log.Printf("Hundreds:%d, Tens:%d, Units:%d\n", hundreds, tens, units)
		if hundreds > 0 {
			words = append(words, slcUnits[hundreds], "hundred")
		}
		if tens == 0 && units == 0 {
			goto hybridEnd
		}

		switch tens {
		case 0:
			words = append(words, slcUnits[units])
		case 1:
			words = append(words, slcTeens[units])
			break
		default:
			if units > 0 {
				word := fmt.Sprintf("%s-%s", slcTens[tens], slcUnits[units])
				words = append(words, word)
			} else {
				words = append(words, slcTens[tens])
			}
			break
		}

	hybridEnd:
		// mega
		if mega := indianMuliplier[idx]; mega != "" {
			words = append(words, mega)
		}
	}
	//log.Printf("Words length: %d\n", len(words))
	return strings.Join(words, " ")
}

// integerToDHybrid converts an integer to a hybrid representation.
//
// The function takes an integer as a parameter and converts it to a hybrid representation.
// It returns a slice of integers.
func integerToDHybrid(number int) []int {
	hybrid := []int{}

	startHybrid := false
	for number > 0 {
		if !startHybrid {
			hybrid = append(hybrid, number%MAX_MULTIPLIER_ALLOWED)
			number = number / MAX_MULTIPLIER_ALLOWED
			startHybrid = true
		} else {
			hybrid = append(hybrid, number%100)
			number = number / MIN_MULTIPLIER_ALLOWED
		}
	}
	//fmt.Println("else hybrid", hybrid)
	return hybrid
}

// IntegerToEnUs converts an integer to international words.
//
// Parameters:
// - input: the integer to convert.
//
// Returns:
// - the English representation of the input integer as a string.
func IntegerToEnUs(input int) string {
	//log.Printf("Input: %d\n", input)
	words := []string{}

	if input < 0 {
		words = append(words, "minus")
		input *= -1
	}

	// split integer in triplets
	triplets := integerToTriplets(input)
	//log.Printf("Triplets: %v\n", triplets)

	// zero is a special case
	if len(triplets) == 0 {
		return "zero"
	}

	// iterate over triplets
	for idx := len(triplets) - 1; idx >= 0; idx-- {
		triplet := triplets[idx]
		//log.Printf("Triplet: %d (idx=%d)\n", triplet, idx)

		// nothing todo for empty triplet
		if triplet == 0 {
			continue
		}

		// three-digits
		hundreds := triplet / MIN_MULTIPLIER_ALLOWED % MIN_TEN_MULTIPLIER
		tens := triplet / MIN_TEN_MULTIPLIER % MIN_TEN_MULTIPLIER
		units := triplet % MIN_TEN_MULTIPLIER
		//log.Printf("Hundreds:%d, Tens:%d, Units:%d\n", hundreds, tens, units)
		if hundreds > 0 {
			words = append(words, slcUnits[hundreds], "hundred")
		}

		if tens == 0 && units == 0 {
			goto tripletEnd
		}

		switch tens {
		case 0:
			words = append(words, slcUnits[units])
		case 1:
			words = append(words, slcTeens[units])
			break
		default:
			if units > 0 {
				word := fmt.Sprintf("%s-%s", slcTens[tens], slcUnits[units])
				words = append(words, word)
			} else {
				words = append(words, slcTens[tens])
			}
			break
		}

	tripletEnd:
		// mega
		if mega := englishMegas[idx]; mega != "" {
			words = append(words, mega)
		}
	}
	//log.Printf("Words length: %d\n", len(words))
	return strings.Join(words, " ")
}

// integerToTriplets generates triplets from the given number.
//
// It takes an integer parameter 'number' and returns a slice of integers.
func integerToTriplets(number int) []int {
	triplets := []int{}

	for number > 0 {
		triplets = append(triplets, number%MAX_MULTIPLIER_ALLOWED)
		number = number / MAX_MULTIPLIER_ALLOWED
	}
	return triplets
}
