package src

import (
	"fmt"
	util "go-framework/util"
	"log"
	"testing"
)

// TestConvertNum2WordsIND calls util.ConvertIntegerToEnIn with a valid string, checking
// for a valid input, it return Indian currency modern words value.
func TestConvertNum2WordsIND(t *testing.T) {
	num := "1234567890123456789.3450"
	msg, err := util.ConvertIntegerToEnIn(num) //ConvertNums2Words(num, "IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestConvertNum2WordsIND(%v), Indian Modern words: %v\n", num, msg)
}

// TestConvertNum2WordsAncientIND calls util.ConvertIntegerToEnAncientIn with a valid string, checking
// for a valid input, it return Indian currency ancient words value.
func TestConvertNum2WordsAncientIND(t *testing.T) {
	num := "1234567890123456789.3450"
	msg, err := util.ConvertIntegerToEnAncientIn(num) //ConvertNums2Words(num, "IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestConvertNum2WordsAncientIND(%v), Indian Ancient words: %v\n", num, msg)
}

// TestConvertNum2WordsINTER calls util.ConvertIntegerToEnUS with a valid string, checking
// for a valid input, it return International currency words value.
func TestConvertNum2WordsINTER(t *testing.T) {
	num := "1234567890123456789.3450"
	msg, err := util.ConvertIntegerToEnUS(num) // ConvertNums2Words(num, "INTERNATIONAL")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestConvertNum2WordsINTER(%v), International words: %v\n", num, msg)
}

// TestConvertNum2WordsEmptyIND calls util.ConvertIntegerToEnIn with an empty string,
// checking for an error.
func TestConvertNum2WordsEmptyIND(t *testing.T) {
	msg, err := util.ConvertIntegerToEnIn("")
	if msg != "" || err == nil {
		t.Fatalf(`TestConvertNum2WordsEmptyIND("") = %q, %v, want "", error\n`, msg, err)
	}
}

// TestConvertNum2WordsEmptyAncientIND calls util.ConvertIntegerToEnAncientIn with an empty string,
// checking for an error.
func TestConvertNum2WordsEmptyAncientIND(t *testing.T) {
	msg, err := util.ConvertIntegerToEnAncientIn("")
	if msg != "" || err == nil {
		t.Fatalf(`TestConvertNum2WordsEmptyAncientIND("") = %q, %v, want "", error\n`, msg, err)
	}
}

// TestConvertNum2WordsEmptyINTER calls util.ConvertNums2Words with an empty string,
// checking for an error.
func TestConvertNum2WordsEmptyINTER(t *testing.T) {
	msg, err := util.ConvertIntegerToEnUS("")
	if msg != "" || err == nil {
		t.Fatalf(`TestConvertNum2WordsEmptyINTER("") = %q, %v, want "", error\n`, msg, err)
	}
}

// TestConvertNum2WordsMinusIND calls util.ConvertIntegerToEnIn with a valid string, checking
// for a valid input, it return Indian currency words value.
func TestConvertNum2WordsMinusIND(t *testing.T) {
	num := "-12345677890.1230"
	msg, err := util.ConvertIntegerToEnIn(num)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestConvertNum2WordsMinusIND(%v), Indian words: %v\n", num, msg)
}

// TestConvertNum2WordsMinusIND calls util.ConvertIntegerToEnAncientIn with a valid string, checking
// for a valid input, it return Indian currency words value.
func TestConvertNum2WordsMinusAncientIND(t *testing.T) {
	num := "-12345677890.1230"
	msg, err := util.ConvertIntegerToEnAncientIn(num)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestConvertNum2WordsMinusAncientIND(%v), Indian words: %v\n", num, msg)
}

// TestConvertNum2WordsMinusINTER calls util.ConvertIntegerToEnUS with a valid string, checking
// for a valid input, it return International currency words value.
func TestConvertNum2WordsMinusINTER(t *testing.T) {
	num := "-12345677890.1230"
	msg, err := util.ConvertIntegerToEnUS(num)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestConvertNum2WordsMinusINTER(%v), International words: %v\n", num, msg)
}

// ExampleTestConvertNum2WordsMinusIND calls util.ConvertIntegerToEnIn with a valid string, checking
// for a valid input, it return Indian currency words value.
func ExampleTestConvertNum2WordsMinusIND() {
	num := "-12345456.84556"
	msg, err := util.ConvertIntegerToEnIn(num)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ExampleTestConvertNum2WordsMinusIND(%v), Indian words: %v\n", num, msg)
}

// ExampleTestConvertNum2WordsMinusAncientIND calls util.ConvertIntegerToEnAncientIn with a valid string, checking
// for a valid input, it return Indian currency words value.
func ExampleTestConvertNum2WordsMinusAncientIND() {
	num := "-12345456.84556"
	msg, err := util.ConvertIntegerToEnAncientIn(num)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ExampleTestConvertNum2WordsMinusAncientIND(%v), Ancient India words: %v\n", num, msg)
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

// TestNumbersWithCommaIND is a test function for the ConvertIntegerToEnInWithComma function in the util package.
//
// This function tests the ConvertIntegerToEnInComma function by passing a number as a string and checking the result.
// It prints the output of the function to the console.
// The function expects the number to be in a specific format and will return an error if the conversion fails.
// The function doesn't return any value, it only prints the output.
func TestNumbersWithCommaIND(t *testing.T) {
	num := "1234567890123456789.3450"
	msg, err := util.ConvertIntegerToEnInWithComma(num) //ConvertNums2Words(num, "IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestNumbersWithCommaIND(%v), Indian Modern with Commas: %v\n", num, msg)
}

// TestNumbersWithCommaAncientIND tests the function ConvertIntegerToEnAncientInWithComma.
//
// It takes a number as input and converts it to its Indian Ancient words representation.
// It prints the result and returns an error if any.
func TestNumbersWithCommaAncientIND(t *testing.T) {
	num := "1234567890123456789.3450"
	msg, err := util.ConvertIntegerToEnAncientInWithComma(num) //ConvertNums2Words(num, "IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestNumbersWithCommaAncientIND(%v), Indian Ancient with Commas: %v\n", num, msg)
}

// TestNumbersWithCommaINTER is a test function for the ConvertIntegerToEnUSWithComma function.
//
// It tests the functionality of converting an integer to a string with comma separators in the en-US format.
// The function takes a number as a string and converts it to a formatted string with comma separators.
// It then prints the converted string as well as the original number in the test output.
//
// Parameters:
// - t: A pointer to the testing.T type provided by the testing package.
//
// Return type:
// None.
func TestNumbersWithCommaINTER(t *testing.T) {
	num := "1234567890123456789.3450"
	msg, err := util.ConvertIntegerToEnUSWithComma(num) // ConvertNums2Words(num, "INTERNATIONAL")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestNumbersWithCommaINTER(%v), International with Commas: %v\n", num, msg)
}

// TestMinusNumbersWithCommaIND is a unit test function for the util.ConvertIntegerToEnInWithComma function.
//
// It tests the conversion of a negative number with commas to Indian modern words.
// The function takes no parameters.
// It returns the converted number in Indian modern words and an error, if any.
func TestMinusNumbersWithCommaIND(t *testing.T) {
	num := "-1234567890123456789.3450"
	msg, err := util.ConvertIntegerToEnInWithComma(num) //ConvertNums2Words(num, "IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestMinusNumbersWithCommaIND(%v), Indian Modern with commas: %v\n", num, msg)
}

// TestMinusNumbersWithCommaAncientIND is a test function that checks the conversion of a negative number with commas to Indian Ancient words.
//
// The function takes no parameters.
// It does not return anything.
func TestMinusNumbersWithCommaAncientIND(t *testing.T) {
	num := "-1234567890123456789.3450"
	msg, err := util.ConvertIntegerToEnAncientInWithComma(num) //ConvertNums2Words(num, "IND")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestMinusNumbersWithCommaAncientIND(%v), Indian Ancient with commas: %v\n", num, msg)
}

// TestMinusNumbersWithCommaINTER is a test function that tests the ConvertIntegerToEnUSWithComma function.
//
// It initializes a variable `num` with a negative number represented as a string.
// Then it calls the ConvertIntegerToEnUSComma function and assigns the result to `msg` variable.
// If an error occurs during the function call, it logs the error and terminates the program.
// Finally, it prints the test name, the input number, and the converted message.
func TestMinusNumbersWithCommaINTER(t *testing.T) {
	num := "-1234567890123456789.3450"
	msg, err := util.ConvertIntegerToEnUSWithComma(num) // ConvertNums2Words(num, "INTERNATIONAL")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("TestMinusNumbersWithCommaINTER(%v), International with commas: %v\n", num, msg)
}

/*go test - output

PS D:\Merce\GoLang_master\go-framework\testdoc> go test
TestConvertNum2WordsIND(1234567890123456789.3450), Indian Modern words: twelve thousand three hundred forty-five crore sixty-seven lakh eighty-nine thousand twelve crore thirty-four lakh fifty-six thousand seven hundred eighty-nine Rupees and thirty-four Paise
TestConvertNum2WordsAncientIND(1234567890123456789.3450), Indian Ancient words: twelve shankh thirty-four padma fifty-six neel seventy-eight kharab ninety arab twelve crore thirty-four lakh fifty-six thousand seven hundred eighty-nine Rupees and thirty-four Paise
TestConvertNum2WordsINTER(1234567890123456789.3450), International words: one quintillion two hundred thirty-four quadrillion five hundred sixty-seven trillion eight hundred ninety billion one hundred twenty-three million four hundred fifty-six thousand seven hundred eighty-nine
and thirty-four
TestConvertNum2WordsMinusIND(-12345677890.1230), Indian words: minus one thousand two hundred thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
TestConvertNum2WordsMinusAncientIND(-12345677890.1230), Indian words: minus twelve arab thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
TestConvertNum2WordsMinusINTER(-12345677890.1230), International words: minus twelve billion three hundred forty-five million six hundred seventy-seven thousand eight hundred ninety and twelve

TestNumbersWithCommaIND(1234567890123456789.3450), Indian Modern with Commas: 12,3,45,67,89,0,12,34,56,789.3450
TestNumbersWithCommaAncientIND(1234567890123456789.3450), Indian Ancient with Commas: 12,34,56,78,90,12,34,56,789.3450
TestNumbersWithCommaINTER(1234567890123456789.3450), International with Commas: 1,234,567,890,123,456,789.3450
TestMinusNumbersWithCommaIND(-1234567890123456789.3450), Indian Modern with commas: -12,3,45,67,89,0,12,34,56,789.3450
TestMinusNumbersWithCommaAncientIND(-1234567890123456789.3450), Indian Ancient with commas: -12,34,56,78,90,12,34,56,789.3450
TestMinusNumbersWithCommaINTER(-1234567890123456789.3450), International with commas: -1,234,567,890,123,456,789.3450
PASS
ok      go-framework/testdoc    0.407s
PASS
ok      go-framework/testdoc    0.678s


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


PS D:\Merce\GoLang_master\go-framework\testdoc> go test -v
=== RUN   TestConvertNum2WordsIND
TestConvertNum2WordsIND(1234567890123456789.3450), Indian Modern words: twelve thousand three hundred forty-five crore sixty-seven lakh eighty-nine thousand twelve crore thirty-four lakh fifty-six thousand seven hundred eighty-nine Rupees and thirty-four Paise
--- PASS: TestConvertNum2WordsIND (0.00s)
=== RUN   TestConvertNum2WordsAncientIND
TestConvertNum2WordsAncientIND(1234567890123456789.3450), Indian Ancient words: twelve shankh thirty-four padma fifty-six neel seventy-eight kharab ninety arab twelve crore thirty-four lakh fifty-six thousand seven hundred eighty-nine Rupees and thirty-four Paise
--- PASS: TestConvertNum2WordsAncientIND (0.00s)
=== RUN   TestConvertNum2WordsINTER
TestConvertNum2WordsINTER(1234567890123456789.3450), International words: one quintillion two hundred thirty-four quadrillion five hundred sixty-seven trillion eight hundred ninety billion one hundred twenty-three million four hundred fifty-six thousand seven hundred eighty-nine
and thirty-four
--- PASS: TestConvertNum2WordsINTER (0.00s)
=== RUN   TestConvertNum2WordsEmptyIND
--- PASS: TestConvertNum2WordsEmptyIND (0.00s)
=== RUN   TestConvertNum2WordsEmptyAncientIND
--- PASS: TestConvertNum2WordsEmptyAncientIND (0.00s)
=== RUN   TestConvertNum2WordsEmptyINTER
--- PASS: TestConvertNum2WordsEmptyINTER (0.00s)
=== RUN   TestConvertNum2WordsMinusIND
TestConvertNum2WordsMinusIND(-12345677890.1230), Indian words: minus one thousand two hundred thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
--- PASS: TestConvertNum2WordsMinusIND (0.00s)
=== RUN   TestConvertNum2WordsMinusAncientIND
TestConvertNum2WordsMinusAncientIND(-12345677890.1230), Indian words: minus twelve arab thirty-four crore fifty-six lakh seventy-seven thousand eight hundred ninety Rupees and twelve Paise
--- PASS: TestConvertNum2WordsMinusAncientIND (0.00s)
=== RUN   TestConvertNum2WordsMinusINTER
TestConvertNum2WordsMinusINTER(-12345677890.1230), International words: minus twelve billion three hundred forty-five million six hundred seventy-seven thousand eight hundred ninety and twelve
--- PASS: TestConvertNum2WordsMinusINTER (0.00s)

=== RUN   TestNumbersWithCommaIND
TestNumbersWithCommaIND(1234567890123456789.3450), Indian Modern with Commas: 12,3,45,67,89,0,12,34,56,789.3450
--- PASS: TestNumbersWithCommaIND (0.00s)
=== RUN   TestNumbersWithCommaAncientIND
TestNumbersWithCommaAncientIND(1234567890123456789.3450), Indian Ancient with Commas: 12,34,56,78,90,12,34,56,789.3450
--- PASS: TestNumbersWithCommaAncientIND (0.00s)
=== RUN   TestNumbersWithCommaINTER
TestNumbersWithCommaINTER(1234567890123456789.3450), International with Commas: 1,234,567,890,123,456,789.3450
--- PASS: TestNumbersWithCommaINTER (0.00s)
=== RUN   TestMinusNumbersWithCommaIND
TestMinusNumbersWithCommaIND(-1234567890123456789.3450), Indian Modern with commas: -12,3,45,67,89,0,12,34,56,789.3450
--- PASS: TestMinusNumbersWithCommaIND (0.00s)
=== RUN   TestMinusNumbersWithCommaAncientIND
TestMinusNumbersWithCommaAncientIND(-1234567890123456789.3450), Indian Ancient with commas: -12,34,56,78,90,12,34,56,789.3450
--- PASS: TestMinusNumbersWithCommaAncientIND (0.00s)
=== RUN   TestMinusNumbersWithCommaINTER
TestMinusNumbersWithCommaINTER(-1234567890123456789.3450), International with commas: -1,234,567,890,123,456,789.3450
--- PASS: TestMinusNumbersWithCommaINTER (0.00s)
PASS
ok      go-framework/testdoc    0.367s
PASS
ok      go-framework/testdoc    0.621s
*/
