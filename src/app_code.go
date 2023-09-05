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
	fmt.Printf("Please enter the numer :")
	fmt.Scan(&input_num)

	words, err := util.ConvertNums2Words(input_num)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("In words: %s", words)
}

/* output

PS D:\Merce\GoLang\go-framework\src> go run .\app_code.go
Please enter the numer :230399
In words: two hundred and thirty thousand three hundred and ninety-nine

*/
