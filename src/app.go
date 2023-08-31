package main

import (
	"fmt"

	valid "go-framework/util"
)

func main() {
	fmt.Println("in main")
	fmt.Println(valid.CleanerTest("clean file"))
	fmt.Println(valid.ValidatorTest("validator file"))

}
