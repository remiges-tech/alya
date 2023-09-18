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
	words, err := util.ConvertIntegerToEnIn(input_num) //Indian words
	if err != nil {
		log.Fatalf("Not a valid number. Error: %v", err)
	}
	fmt.Printf("In words(Modern India): %s\n", words)

	ancient_words, err := util.ConvertIntegerToEnAncientIn(input_num) //Indian words
	if err != nil {
		log.Fatalf("Not a valid number. Error: %v", err)
	}
	fmt.Printf("In words(Ancient India): %s\n", ancient_words)

	words1, err := util.ConvertIntegerToEnUS(input_num) //INTERNATIONAL words
	if err != nil {
		log.Fatalf("Not a valid number. Error: %v", err)
	}
	fmt.Printf("In words(International): %s\n", words1)

	/*words, err := util.ConvertNums2Words(input_num,"IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("In words: %s", words)*/
}

/* output

PS D:\Merce\GoLang_master\go-framework\src> go run .\app_code.go
Please enter the number :1234567890123456789.123
In words(Modern India): twelve thousand three hundred forty-five crore sixty-seven lakh eighty-nine thousand twelve crore thirty-four lakh fifty-six thousand seven hundred eighty-nine Rupees and twelve Paise
In words(Ancient India): twelve shankh thirty-four padma fifty-six neel seventy-eight kharab ninety arab twelve crore thirty-four lakh fifty-six thousand seven hundred eighty-nine Rupees and twelve Paise
In words(International): one quintillion two hundred thirty-four quadrillion five hundred sixty-seven trillion eight hundred ninety billion
one hundred twenty-three million four hundred fifty-six thousand seven hundred eighty-nine and twelve
PS D:\Merce\GoLang_master\go-framework\src> go run .\app_code.go
Please enter the number :-1234567890123456789.123
In words(Modern India): minus twelve thousand three hundred forty-five crore sixty-seven lakh eighty-nine thousand twelve crore thirty-four
lakh fifty-six thousand seven hundred eighty-nine Rupees and twelve Paise
In words(Ancient India): minus twelve shankh thirty-four padma fifty-six neel seventy-eight kharab ninety arab twelve crore thirty-four lakh fifty-six thousand seven hundred eighty-nine Rupees and twelve Paise
In words(International): minus one quintillion two hundred thirty-four quadrillion five hundred sixty-seven trillion eight hundred ninety billion one hundred twenty-three million four hundred fifty-six thousand seven hundred eighty-nine and twelve


PS D:\Merce\GoLang_master\go-framework\src> go run .\app_code.go
Please enter the number :12345678901234567890.123
2023/09/18 13:59:29 Not a valid number. Error: Overflow error: 19 digits are allowed max. for example: 1234567890123456789.03
exit status 1

PS D:\Merce\GoLang_master\go-framework\src> go run .\app_code.go
Please enter the number :-12345678901234567890.123
2023/09/18 14:00:02 Not a valid number. Error: Overflow error: 19 digits are allowed max. for example: 1234567890123456789.03
exit status 1

*/
