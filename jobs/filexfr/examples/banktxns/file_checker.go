package main

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/remiges-tech/alya/jobs"
)

// checkBankTransactionFile processes a CSV file of bank transactions.
// It is part of the file transfer (filexfr) system in our bank transaction example.
// This function does the following:
// 1. Reads the CSV file contents.
// 2. Validates each transaction record.
// 3. Converts each valid record into a JSON format.
// 4. Prepares a batch of transactions for processing.
// 5. Returns information needed by the filexfr system to handle the file.
//
// The function is called by the filexfr system when a new CSV file is received.
func checkBankTransactionFile(fileContents string, fileName string) (bool, jobs.JSONstr, []jobs.BatchInput_t, string, string, string) {
	reader := csv.NewReader(strings.NewReader(fileContents))
	records, err := reader.ReadAll()
	if err != nil {
		return false, jobs.JSONstr{}, nil, "", "", ""
	}

	var batchInput []jobs.BatchInput_t
	for i, record := range records {
		if len(record) != 3 {
			return false, jobs.JSONstr{}, nil, "", "", ""
		}

		amount, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			return false, jobs.JSONstr{}, nil, "", "", ""
		}

		// Add a validation check
		if amount <= 0 {
			return false, jobs.JSONstr{}, nil, "", "", ""
		}

		transaction := Transaction{
			ID:     record[0],
			Type:   record[1],
			Amount: amount,
		}

		jsonStr, err := jobs.NewJSONstr(fmt.Sprintf(`{"id": "%s", "type": "%s", "amount": %.2f}`, transaction.ID, transaction.Type, transaction.Amount))
		if err != nil {
			return false, jobs.JSONstr{}, nil, "", "", ""
		}

		batchInput = append(batchInput, jobs.BatchInput_t{
			Line:  i + 1,
			Input: jsonStr,
		})
	}

	context, _ := jobs.NewJSONstr(`{"filename": "` + fileName + `"}`)
	return true, context, batchInput, "bankapp", "processtransactions", ""
}
