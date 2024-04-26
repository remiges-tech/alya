package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
)

func main() {
	numTransactions := 1000
	outputFile := "transactions.csv"

	file, err := os.Create(outputFile)
	if err != nil {
		log.Fatal("Error creating output file:", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for i := 1; i <= numTransactions; i++ {
		transactionType := "DEPOSIT"
		if rand.Float64() < 0.5 {
			transactionType = "WITHDRAWAL"
		}

		amount := rand.Float64() * 1000 // Random amount between 0 and 1000

		record := []string{
			fmt.Sprintf("TX%04d", i),
			transactionType,
			strconv.FormatFloat(amount, 'f', 2, 64),
		}

		err := writer.Write(record)
		if err != nil {
			log.Fatal("Error writing record to CSV:", err)
		}
	}

	fmt.Println("Transaction data generated successfully.")
}
