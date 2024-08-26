package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
)

func main() {
	csvFile :=  "../generate_txn/transactions.csv"

	file, err := os.Open(csvFile)
	if err != nil {
		log.Fatal("Error opening CSV file:", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal("Error reading CSV records:", err)
	}

	balance := 0.0
	for _, record := range records {
		if len(record) != 3 {
			log.Fatal("Invalid CSV record")
		}

		transactionType := record[1]
		amount, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			log.Fatal("Invalid amount:", err)
		}

		if transactionType == "DEPOSIT" {
			balance += amount
		} else if transactionType == "WITHDRAWAL" {
			balance -= amount
		}
	}

	fmt.Printf("Final balance: %.2f\n", balance)
}
