package main

import (
	"fmt"
	util "go-framework/util"
	"log"
)

// This is main function. main is the entry point of the Go program.
// This written to convert Numbers to words
// There are no input parameters.
// It returns strings in return types.
func main() {
	var input_num string
	fmt.Printf("Please enter the number :")
	fmt.Scan(&input_num)

	//get integer to Indian words.
	words, err := util.ConvertIntegerToEnIn(input_num)
	if err != nil {
		log.Fatal("Not a valid number")
	}
	fmt.Printf("In words(India): %s\n", words)

	//get integer to international words
	words1, err := util.ConvertIntegerToEnUS(input_num)
	if err != nil {
		log.Fatal("Not a valid number")
	}
	fmt.Printf("In words(International): %s\n", words1)

	/*words, err := util.ConvertNums2Words(input_num,"IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("In words: %s", words)*/
}

/* output

PS D:\Merce\GoLang\go-framework\src> go run .\app_code.go
Please enter the numer :-12345.400
Entered Number:  -12345.400390625
In words(India): minus twelve thousand three hundred forty-five Rupees and forty Paise
Entered Number:  -12345.400390625
In words(International): minus twelve thousand three hundred forty-five and forty

Please enter the numer :12334434534534545.7000
Entered Number:  1.2334434183282688e+16
In words(India): twelve padma thirty-three neel forty-four kharab thirty-four arab fifty-three crore forty-five lakh thirty-four thousand five hundred forty-five Rupees and seventy Paise
Entered Number:  1.2334434183282688e+16
In words(International): twelve quadrillion three hundred thirty-four trillion four hundred thirty-four billion five hundred thirty-four million five hundred thirty-four thousand five hundred forty-five and seventy

PS D:\Merce\GoLang\go-framework\src> go run .\app_code.go
Please enter the numer :12345678901234567.43445
Entered Number:  1.2345678407663616e+16
In words(India): twelve padma thirty-four neel fifty-six kharab seventy-eight arab ninety crore twelve lakh thirty-four thousand five hundred sixty-seven Rupees and forty-three Paise
Entered Number:  1.2345678407663616e+16
In words(International): twelve quadrillion three hundred forty-five trillion six hundred seventy-eight billion nine hundred one million two hundred thirty-four thousand five hundred sixty-seven and forty-three
*/
