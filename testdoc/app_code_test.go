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
	msg, err := util.ConvertNums2Words(num, "IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg)
}

// TestConvertNum2WordsINTER calls util.ConvertNums2Words with a valid string, checking
// for a valid input, it return Indian currency words value.
func TestConvertNum2WordsINTER(t *testing.T) {
	num := "12345677890.1230"
	msg, err := util.ConvertNums2Words(num, "INTERNATIONAL")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg)
}

// TestConvertNum2WordsEmptyIND calls util.ConvertNums2Words with an empty string,
// checking for an error.
func TestConvertNum2WordsEmptyIND(t *testing.T) {
	msg, err := util.ConvertNums2Words("", "IND")
	if msg != "" || err == nil {
		t.Fatalf(`ConvertNums2Words("") = %q, %v, want "", error`, msg, err)
	}
}

// TestConvertNum2WordsEmptyINTER calls util.ConvertNums2Words with an empty string,
// checking for an error.
func TestConvertNum2WordsEmptyINTER(t *testing.T) {
	msg, err := util.ConvertNums2Words("", "INTERNATIONAL")
	if msg != "" || err == nil {
		t.Fatalf(`ConvertNums2Words("") = %q, %v, want "", error`, msg, err)
	}
}

// TestConvertNum2WordsMinusIND calls util.ConvertNums2Words with a valid string, checking
// for a valid input, it return Indian currency words value.
func TestConvertNum2WordsMinusIND(t *testing.T) {
	num := "-12345677890.1230"
	msg, err := util.ConvertNums2Words(num, "IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg)
}

// TestConvertNum2WordsMinusINTER calls util.ConvertNums2Words with a valid string, checking
// for a valid input, it return Indian currency words value.
func TestConvertNum2WordsMinusINTER(t *testing.T) {
	num := "-12345677890.1230"
	msg, err := util.ConvertNums2Words(num, "INTERNATIONAL")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg)
}

// ExampleTestConvertNum2WordsMinusIND calls util.ConvertNums2Words with a valid string, checking
// for a valid input, it return Indian currency words value.
func ExampleTestConvertNum2WordsMinusIND() {
	num := "-12345456.84556"
	msg, err := util.ConvertNums2Words(num, "IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg)
}

// ExampleTestConvertNum2WordsMinusINTER calls util.ConvertNums2Words with a valid string, checking
// for a valid input, it return Indian currency words value.
func ExampleTestConvertNum2WordsMinusINTER() {
	num := "-12345456.84556"
	msg, err := util.ConvertNums2Words(num, "INTERNATIONAL")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg)
}

/*go test - output

PS D:\Merce\GoLang\go-framework\testdoc> go test
Entered Number:  1.2345677824e+10
twelve arab thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
Entered Number:  1.2345677824e+10
twelve billion three hundred forty-five million six hundred seventy-seven thousand eight hundred ninety and twelve
minus twelve arab thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
Entered Number:  -1.2345677824e+10
minus twelve billion three hundred forty-five million six hundred seventy-seven thousand eight hundred ninety and twelve
PASS
ok      go-framework/testdoc    0.035s


PS D:\Merce\GoLang\go-framework\testdoc> go test -v
=== RUN   TestConvertNum2WordsIND
Entered Number:  1.2345677824e+10
twelve arab thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
--- PASS: TestConvertNum2WordsIND (0.00s)
=== RUN   TestConvertNum2WordsINTER
Entered Number:  1.2345677824e+10
twelve billion three hundred forty-five million six hundred seventy-seven thousand eight hundred ninety and twelve
--- PASS: TestConvertNum2WordsINTER (0.00s)
=== RUN   TestConvertNum2WordsEmptyIND
--- PASS: TestConvertNum2WordsEmptyIND (0.00s)
=== RUN   TestConvertNum2WordsEmptyINTER
--- PASS: TestConvertNum2WordsEmptyINTER (0.00s)
=== RUN   TestConvertNum2WordsMinusIND
Entered Number:  -1.2345677824e+10
minus twelve arab thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
--- PASS: TestConvertNum2WordsMinusIND (0.00s)
=== RUN   TestConvertNum2WordsMinusINTER
Entered Number:  -1.2345677824e+10
minus twelve billion three hundred forty-five million six hundred seventy-seven thousand eight hundred ninety and twelve
--- PASS: TestConvertNum2WordsMinusINTER (0.00s)
PASS
ok      go-framework/testdoc    0.030s

*/
