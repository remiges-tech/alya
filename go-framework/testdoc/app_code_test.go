package src

import (
	"fmt"
	util "go-framework/util"
	"log"
	"testing"
)

// TestConvertNum2Words calls util.ConvertNums2Words with a valid string, checking
// for a valid return value.
func TestConvertNum2Words(t *testing.T) {
	num := "1234"
	msg, err := util.ConvertNums2Words(num)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg)

}

// TestConvertNum2WordsEmpty calls util.ConvertNums2Words with an empty string,
// checking for an error.
func TestConvertNum2WordsEmpty(t *testing.T) {
	msg, err := util.ConvertNums2Words("")
	if msg != "" || err == nil {
		t.Fatalf(`ConvertNums2Words("") = %q, %v, want "", error`, msg, err)
	}
}

// TestConvertNum2WordsMinus calls util.ConvertNums2Words with a valid string, checking
// for a valid return value.
func TestConvertNum2WordsMinus(t *testing.T) {
	num := "-1234"
	msg, err := util.ConvertNums2Words(num)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg)

}

/*go test - output

PS D:\Merce\GoLang\go-framework> cd .\testdoc\
PS D:\Merce\GoLang\go-framework\testdoc> go test
one thousand two hundred and thirty-four
minus one thousand two hundred and thirty-four
PASS
ok      go-framework/testdoc    0.024s


PS D:\Merce\GoLang\go-framework\testdoc> go test -v
=== RUN   TestConvertNum2Words
one thousand two hundred and thirty-four
--- PASS: TestConvertNum2Words (0.00s)
=== RUN   TestConvertNum2WordsEmpty
--- PASS: TestConvertNum2WordsEmpty (0.00s)
=== RUN   TestConvertNum2WordsMinus
minus one thousand two hundred and thirty-four
--- PASS: TestConvertNum2WordsMinus (0.00s)
PASS
ok      go-framework/testdoc    0.023s
PS D:\Merce\GoLang\go-framework\testdoc>

*/
