package src

import (
	"fmt"
	util "go-framework/util"
	"log"
	"testing"
)

// TestConvertNum2WordsIND calls util.ConvertNums2Words with a valid string, checking
// for a valid input, it return Indian currency words value.
func TestConvertNum2WordsIND(t *testing.T) {
	num := "12345677890.1230"
	msg, err := util.ConvertIntegerToEnIn(num) //ConvertNums2Words(num, "IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestConvertNum2WordsIND(%v), Indian words: %v\n", num, msg)
}

// TestConvertNum2WordsINTER calls util.ConvertNums2Words with a valid string, checking
// for a valid input, it return International currency words value.
func TestConvertNum2WordsINTER(t *testing.T) {
	num := "12345677890.1230"
	msg, err := util.ConvertIntegerToEnUS(num) // ConvertNums2Words(num, "INTERNATIONAL")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestConvertNum2WordsINTER(%v), International words: %v\n", num, msg)
}

// TestConvertNum2WordsEmptyIND calls util.ConvertNums2Words with an empty string,
// checking for an error.
func TestConvertNum2WordsEmptyIND(t *testing.T) {
	msg, err := util.ConvertIntegerToEnIn("")
	if msg != "" || err == nil {
		t.Fatalf(`TestConvertNum2WordsEmptyIND("") = %q, %v, want "", error`, msg, err)
	}
}

// TestConvertNum2WordsEmptyINTER calls util.ConvertNums2Words with an empty string,
// checking for an error.
func TestConvertNum2WordsEmptyINTER(t *testing.T) {
	msg, err := util.ConvertIntegerToEnUS("")
	if msg != "" || err == nil {
		t.Fatalf(`TestConvertNum2WordsEmptyINTER("") = %q, %v, want "", error`, msg, err)
	}
}

// TestConvertNum2WordsMinusIND calls util.ConvertNums2Words with a valid string, checking
// for a valid input, it return Indian currency words value.
func TestConvertNum2WordsMinusIND(t *testing.T) {
	num := "-12345677890.1230"
	msg, err := util.ConvertIntegerToEnIn(num)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestConvertNum2WordsMinusIND(%v), Indian words: %v\n", num, msg)
}

// TestConvertNum2WordsMinusINTER calls util.ConvertNums2Words with a valid string, checking
// for a valid input, it return International currency words value.
func TestConvertNum2WordsMinusINTER(t *testing.T) {
	num := "-12345677890.1230"
	msg, err := util.ConvertIntegerToEnUS(num)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestConvertNum2WordsMinusINTER(%v), International words: %v\n", num, msg)
}

// ExampleTestConvertNum2WordsMinusIND calls util.ConvertNums2Words with a valid string, checking
// for a valid input, it return Indian currency words value.
func ExampleTestConvertNum2WordsMinusIND() {
	num := "-12345456.84556"
	msg, err := util.ConvertIntegerToEnIn(num)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ExampleTestConvertNum2WordsMinusIND(%v), India words: %v\n", num, msg)
}

// ExampleTestConvertNum2WordsMinusINTER calls util.ConvertNums2Words with a valid string, checking
// for a valid input, it return International currency words value.
func ExampleTestConvertNum2WordsMinusINTER() {
	num := "-12345456.84556"
	msg, err := util.ConvertIntegerToEnUS(num)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ExampleTestConvertNum2WordsMinusINTER(%v), International words: %v\n", num, msg)
}

/*go test - output

PS D:\Merce\GoLang\go-framework\testdoc> go test
Entered Number:  1.2345677824e+10
TestConvertNum2WordsIND(12345677890.1230), Indian words: twelve arab thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
Entered Number:  1.2345677824e+10
TestConvertNum2WordsINTER(12345677890.1230), International words: twelve billion three hundred forty-five million six hundred seventy-seven
thousand eight hundred ninety and twelve
Entered Number:  -1.2345677824e+10
TestConvertNum2WordsMinusIND(-12345677890.1230), Indian words: minus twelve arab thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
Entered Number:  -1.2345677824e+10
TestConvertNum2WordsMinusINTER(-12345677890.1230), International words: minus twelve billion three hundred forty-five million six hundred seventy-seven thousand eight hundred ninety and twelve
PASS
ok      go-framework/testdoc    0.035s


PS D:\Merce\GoLang\go-framework\testdoc> go test -v
=== RUN   TestConvertNum2WordsIND
Entered Number:  1.2345677824e+10
TestConvertNum2WordsIND(12345677890.1230), Indian words: twelve arab thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
--- PASS: TestConvertNum2WordsIND (0.00s)
=== RUN   TestConvertNum2WordsINTER
Entered Number:  1.2345677824e+10
TestConvertNum2WordsINTER(12345677890.1230), International words: twelve billion three hundred forty-five million six hundred seventy-seven
thousand eight hundred ninety and twelve
--- PASS: TestConvertNum2WordsINTER (0.00s)
=== RUN   TestConvertNum2WordsEmptyIND
--- PASS: TestConvertNum2WordsEmptyIND (0.00s)
=== RUN   TestConvertNum2WordsEmptyINTER
--- PASS: TestConvertNum2WordsEmptyINTER (0.00s)
=== RUN   TestConvertNum2WordsMinusIND
Entered Number:  -1.2345677824e+10
TestConvertNum2WordsMinusIND(-12345677890.1230), Indian words: minus twelve arab thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
--- PASS: TestConvertNum2WordsMinusIND (0.00s)
=== RUN   TestConvertNum2WordsMinusINTER
Entered Number:  -1.2345677824e+10
TestConvertNum2WordsMinusINTER(-12345677890.1230), International words: minus twelve billion three hundred forty-five million six hundred seventy-seven thousand eight hundred ninety and twelve
--- PASS: TestConvertNum2WordsMinusINTER (0.00s)
PASS
ok      go-framework/testdoc    0.034s

*/
