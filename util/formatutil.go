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

// Ancient Indian Muilitpliers or Units
var indianMuliplier = []string{"", "thousand", "lakh", "crore", "arab", "kharab", "neel", "padma", "shankh", "mahashankh"}

// Modern Indian Muilitpliers or Units
var indianMuliplier2 = []string{"", "thousand", "lakh", "crore", "hundred", "thousand", "lakh", "crore", "hundred", "thousand"}

// International  Muilitpliers or Units
var englishMegas = []string{"", "thousand", "million", "billion", "trillion", "quadrillion", "quintillion", "sextillion", "septillion", "octillion", "nonillion", "decillion", "undecillion", "duodecillion", "tredecillion", "quattuordecillion"}

// const variables
const (
	//coonstant value for max number of digits allowed
	MAX_NUMBERS_ALLOWED = 19
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

// ConvertIntegerToEnAncientIn converts an integer to its English representation in Ancient Indian format.
//
// Parameters:
// - input_num: the input number as a string.
//
// Returns:
// - the Ancient Indian English representation of the number, and the error is returned if the input_num is empty or not a valid number.
func ConvertIntegerToEnAncientIn(input_num string) (string, error) {
	// If no input_num was given, return an error with a message.
	if input_num == "" {
		return input_num, errors.New("Empty string. Please provide valid number")
	}

	_, err := strconv.ParseFloat(input_num, 32)
	if err != nil {
		return input_num, errors.New("Not a valid number")
	}
	slc := []string{}
	slc = strings.Split(input_num, ".")
	words := []string{}

	for i := 0; i < len(slc); i++ {

		temp_str1 := ""
		temp_str1 = slc[i][MIN_DECIMAL_NUMBER_ALLOWED:1]
		//fmt.Println(temp_str1)
		if (temp_str1 == "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED+1) || (temp_str1 != "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED) {
			return input_num, errors.New("Overflow error: 19 digits are allowed max. for example: 1234567890123456789.03")
		}

		number, err := strconv.Atoi(slc[i])
		if err != nil {
			return input_num, errors.New("Not a valid number")
		}
		// Create a response using a string, error format.

		// if its valid number then it will return proper string
		//approach 1 will accept 12 digit numbers
		if i == 0 {
			words = append(words, IntegerToEnAncientIn(number))
			words = append(words, "Rupees")
		} else {
			if number >= MIN_MULTIPLIER_ALLOWED {
				temp_str := slc[i]
				//consider first two digits
				temp_str1 := temp_str[MIN_DECIMAL_NUMBER_ALLOWED:MAX_DECIMAL_NUMBER_ALLOWED]
				number, _ = strconv.Atoi(temp_str1)
			}
			words = append(words, "and "+IntegerToEnAncientIn(number)+" Paise")
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

	_, err := strconv.ParseFloat(input_num, 64)
	if err != nil {
		return input_num, errors.New("Not a valid number")
	}
	slc := []string{}
	slc = strings.Split(input_num, ".")
	words := []string{}

	for i := 0; i < len(slc); i++ {
		temp_str1 := ""
		temp_str1 = slc[i][MIN_DECIMAL_NUMBER_ALLOWED:1]
		//fmt.Println(temp_str1)
		if (temp_str1 == "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED+1) || (temp_str1 != "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED) {
			return input_num, errors.New("Overflow error: 19 digits are allowed max. for example: 1234567890123456789.03")
		}

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

// ConvertIntegerToEnIn converts an integer to its English representation in Indian numbering system.
//
// input_num: the input number as a string.
// Returns the English representation of the input number and an error, if any.
func ConvertIntegerToEnIn(input_num string) (string, error) {
	// If no input_num was given, return an error with a message.
	if input_num == "" {
		return input_num, errors.New("Empty string. Please provide valid number")
	}
	_, err := strconv.ParseFloat(input_num, 64)
	if err != nil {
		return input_num, errors.New("Not a valid number1")
	}
	slc := []string{}
	slc = strings.Split(input_num, ".")
	words := []string{}

	for i := 0; i < len(slc); i++ {
		temp_str1 := ""
		temp_str1 = slc[i][MIN_DECIMAL_NUMBER_ALLOWED:1]
		if (temp_str1 == "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED+1) || (temp_str1 != "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED) {
			return input_num, errors.New("Overflow error: 19 digits are allowed max. for example: 1234567890123456789.03")
		}
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

// IntegerToEnAncientIn converts an integer to indian words.
//
// Parameters:
// - input: the integer to convert.
//
// Returns:
// - the English representation of the input integer as a string.
func IntegerToEnAncientIn(input int) string {
	//log.Printf("Input: %d\n", input)
	words := []string{}

	if input < 0 {
		words = append(words, "minus")
		input *= -1
	}

	// split integer in hybrids
	var hybrids []int
	hybrids = integerToDAncinetHybrid(input)
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
		if mega := indianMuliplier2[idx]; mega != "" {
			words = append(words, mega)
		}
	}
	//log.Printf("Words length: %d\n", len(words))
	return strings.Join(words, " ")
}

// integerToDAncinetHybrid converts an integer to a hybrid representation.
//
// The function takes an integer as a parameter and converts it to a hybrid representation.
// It returns a slice of integers.
func integerToDAncinetHybrid(number int) []int {
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

// integerToDModernHybrid converts an integer to a hybrid representation.
//
// The function takes an integer as a parameter and converts it to a hybrid representation.
// It returns a slice of integers.
func integerToDHybrid(number int) []int {
	hybrid := []int{}
	i := 0
	startHybrid := false
	for number > 0 {
		i += 1
		if !startHybrid {
			hybrid = append(hybrid, number%MAX_MULTIPLIER_ALLOWED)
			number = number / MAX_MULTIPLIER_ALLOWED
			startHybrid = true
		} else if i == 5 || i == 9 {
			hybrid = append(hybrid, number%10)
			number = number / 10
		} else {
			hybrid = append(hybrid, number%100)
			number = number / MIN_MULTIPLIER_ALLOWED
		}
	}
	return hybrid
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

// ConvertIntegerToEnAncientInWithComma converts an integer to an English ancient representation in comma format.
//
// It takes a string `input_num` as input and returns a string and an error. The `input_num` represents the number to be converted.
// The function returns the ancient representation of the number in comma format and an error, if any.
func ConvertIntegerToEnAncientInWithComma(input_num string) (string, error) {
	// If no input_num was given, return an error with a message.
	if input_num == "" {
		return input_num, errors.New("Empty string. Please provide valid number")
	}
	//log.Println(ntw.IntegerToEnIn().IntegerToEnIn(1234567890))

	_, err := strconv.ParseFloat(input_num, 64)
	if err != nil {
		return input_num, errors.New("Not a valid number1")
	}
	slc := []string{}
	slc = strings.Split(input_num, ".")
	words := []string{}

	for i := 0; i < len(slc); i++ {
		temp_str1 := ""
		temp_str1 = slc[i][MIN_DECIMAL_NUMBER_ALLOWED:1]
		if (temp_str1 == "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED+1) || (temp_str1 != "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED) {
			return input_num, errors.New("Overflow error: 19 digits are allowed max. for example: 1234567890123456789.03")
		}
		number, err := strconv.Atoi(slc[i])
		if err != nil {
			return input_num, errors.New("Not a valid number")
		}
		if i == 0 {
			words = append(words, NumberWithCommaInEnAncientInd(number))

		} else {
			words = append(words, "."+slc[i])

		}
	}
	return strings.Join(words, ""), nil

}

// NumberWithCommaInEnAncientInd converts an integer to its ancient English representation with commas.
//
// It takes an input integer and returns a string representing the ancient English representation of the integer
// with commas.
func NumberWithCommaInEnAncientInd(input int) string {
	//log.Printf("Input: %d\n", input)
	words := []string{}
	if input < 0 {
		words = append(words, "-")
		input *= -1
	}

	// split integer in ancient hybrids
	var hybrids []string
	hybrids = integerToDAncinetHybridComma(input)
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
		if hybrid == "" {
			continue
		}
		if idx > 0 {
			words = append(words, hybrids[idx], ",")
		} else {
			words = append(words, hybrids[idx], "")
		}
	}
	//log.Printf("Words length: %d\n", len(words))
	return strings.Join(words, "")
}

// integerToDAncinetHybridComma generates a hybrid comma-separated representation of an integer.
//
// It takes an integer as a parameter and returns a slice of strings.
func integerToDAncinetHybridComma(number int) []string {
	hybrid := []string{}

	startHybrid := false
	for number > 0 {
		if !startHybrid {
			str := fmt.Sprintf("%03d", number%MAX_MULTIPLIER_ALLOWED)
			hybrid = append(hybrid, str)
			number = number / MAX_MULTIPLIER_ALLOWED
			startHybrid = true
		} else {
			str := fmt.Sprintf("%02d", number%MIN_MULTIPLIER_ALLOWED)
			if len(strconv.Itoa(number)) > 1 {
				hybrid = append(hybrid, str)
			} else {
				hybrid = append(hybrid, strconv.Itoa(number%MIN_MULTIPLIER_ALLOWED))
			}
			number = number / MIN_MULTIPLIER_ALLOWED
		}
	}
	return hybrid
}

// ConvertIntegerToEnInWithComma converts an integer in string format to its English representation with commas.
//
// input_num: The string representation of the input integer.
// Returns the English representation of the input integer with commas as a string and an error, if any.
func ConvertIntegerToEnInWithComma(input_num string) (string, error) {
	// If no input_num was given, return an error with a message.
	if input_num == "" {
		return input_num, errors.New("Empty string. Please provide valid number")
	}
	_, err := strconv.ParseFloat(input_num, 64)
	if err != nil {
		return input_num, errors.New("Not a valid number1")
	}
	slc := []string{}
	slc = strings.Split(input_num, ".")
	words := []string{}

	for i := 0; i < len(slc); i++ {
		temp_str1 := ""
		temp_str1 = slc[i][MIN_DECIMAL_NUMBER_ALLOWED:1]
		if (temp_str1 == "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED+1) || (temp_str1 != "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED) {
			return input_num, errors.New("Overflow error: 19 digits are allowed max. for example: 1234567890123456789.03")
		}
		number, err := strconv.Atoi(slc[i])
		if err != nil {
			return input_num, errors.New("Not a valid number")
		}
		if i == 0 {
			words = append(words, NumberWithCommaInEnInd(number))

		} else {
			words = append(words, "."+slc[i])
		}

	}
	return strings.Join(words, ""), nil
}

// NumberWithCommaInEnInd converts an integer to its English representation with commas.
//
// input: the integer to be converted.
// returns: the English representation of the input integer with commas.
func NumberWithCommaInEnInd(input int) string {
	words := []string{}

	if input < 0 {
		words = append(words, "-")
		input *= -1
	}

	// split integer in hybrids
	var hybrids []string
	hybrids = integerToDHybridComma(input)

	// zero is a special case
	if len(hybrids) == 0 {
		return "zero"
	}

	// iterate over hybrids
	for idx := len(hybrids) - 1; idx >= 0; idx-- {
		hybrid := hybrids[idx]
		//log.Printf("hybrid: %d (idx=%d)\n", hybrid, idx)

		// nothing todo for empty hybrid
		if hybrid == "" {
			continue
		}

		if idx > 0 {
			words = append(words, hybrids[idx], ",")
		} else {
			words = append(words, hybrids[idx])
		}
	}

	//log.Printf("Words length: %d\n", len(words))
	return strings.Join(words, "")
}

// integerToDHybridComma converts an integer to a hybrid comma-separated string.
//
// It takes an integer as input and returns a slice of strings, where each string
// represents a part of the converted number. The converted number is split into
// sections of three digits, except for sections at positions 5 and 9, which are
// split into two digits. The resulting strings are stored in a slice and returned.
func integerToDHybridComma(number int) []string {
	hybrid := []string{}
	i := 0
	startHybrid := false
	for number > 0 {
		i += 1
		if !startHybrid {
			str := fmt.Sprintf("%03d", number%MAX_MULTIPLIER_ALLOWED)
			hybrid = append(hybrid, str)
			number = number / MAX_MULTIPLIER_ALLOWED
			startHybrid = true
		} else if i == 5 || i == 9 {
			hybrid = append(hybrid, strconv.Itoa(number%MIN_TEN_MULTIPLIER))
			number = number / MIN_TEN_MULTIPLIER
		} else {
			str := fmt.Sprintf("%02d", number%MIN_MULTIPLIER_ALLOWED)
			if len(strconv.Itoa(number)) > 1 {
				hybrid = append(hybrid, str)
			} else {
				hybrid = append(hybrid, strconv.Itoa(number%MIN_MULTIPLIER_ALLOWED))
			}
			number = number / MIN_MULTIPLIER_ALLOWED
		}
	}
	return hybrid
}

// ConvertIntegerToEnUSWithComma converts an integer represented as a string into a comma-separated string in US English format.
//
// input_num: The input integer as a string.
// Returns the converted comma-separated string and an error if any.
func ConvertIntegerToEnUSWithComma(input_num string) (string, error) {
	// If no input_num was given, return an error with a message.
	if input_num == "" {
		return input_num, errors.New("Empty string. Please provide valid number")
	}

	_, err := strconv.ParseFloat(input_num, 64)
	if err != nil {
		return input_num, errors.New("Not a valid number")
	}
	slc := []string{}
	slc = strings.Split(input_num, ".")
	words := []string{}

	for i := 0; i < len(slc); i++ {
		temp_str1 := ""
		temp_str1 = slc[i][MIN_DECIMAL_NUMBER_ALLOWED:1]
		if (temp_str1 == "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED+1) || (temp_str1 != "-" && len(slc[i]) > MAX_NUMBERS_ALLOWED) {
			return input_num, errors.New("Overflow error: 19 digits are allowed max. for example: 1234567890123456789.03")
		}
		number, err := strconv.Atoi(slc[i])
		if err != nil {
			return input_num, errors.New("Not a valid number")
		}
		// Create a response using a string, error format.
		if i == 0 {
			words = append(words, NumberWithCommaInEnUs(number))
		} else {
			words = append(words, "."+slc[i])
		}

	}
	return strings.Join(words, ""), nil
}

// NumberWithCommaInEnUs converts an integer to a string representation using the English-US comma format.
//
// It takes an integer input and returns a string.
func NumberWithCommaInEnUs(input int) string {
	//log.Printf("Input: %d\n", input)
	words := []string{}

	if input < 0 {
		words = append(words, "-")
		input *= -1
	}

	// split integer in triplets
	triplets := integerToTripletsComma(input)

	// zero is a special case
	if len(triplets) == 0 {
		return "zero"
	}

	// iterate over triplets
	for idx := len(triplets) - 1; idx >= 0; idx-- {
		if idx > 0 {
			words = append(words, triplets[idx], ",")
		} else {
			//fmt.Println(idx)
			words = append(words, triplets[idx], "")
		}
	}

	//log.Printf("Words length: %d\n", len(words))
	return strings.Join(words, "")
}

// integerToTripletsComma converts an integer into triplets of comma-separated strings.
//
// It takes an integer parameter `number` and returns a slice of strings.
func integerToTripletsComma(number int) []string {
	triplets := []string{}
	for number > 0 {
		str := fmt.Sprintf("%03d", number%MAX_MULTIPLIER_ALLOWED)
		if len(strconv.Itoa(number)) > 2 {
			triplets = append(triplets, str)

		} else {
			triplets = append(triplets, strconv.Itoa(number%MAX_MULTIPLIER_ALLOWED))
		}
		number = number / MAX_MULTIPLIER_ALLOWED

	}
	return triplets
}
