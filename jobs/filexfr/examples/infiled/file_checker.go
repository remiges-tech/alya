package main

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/remiges-tech/alya/jobs"
)

// checkBankTransactionFile processes a CSV file containing bank transactions.
// It validates the file format and content, and prepares data for batch submission (batchInput type).
func checkBankTransactionFile(fileContents string, fileName string) (bool, jobs.JSONstr, []jobs.BatchInput_t, string, string, string) {
	// Create a new CSV reader from the file contents
	reader := csv.NewReader(strings.NewReader(fileContents))

	// Read all the records from the CSV file
	records, err := reader.ReadAll()
	if err != nil {
		// If there is an error reading the CSV, return false indicating the file is invalid
		return false, jobs.JSONstr{}, nil, "", "", ""
	}

	// Initialize a slice to hold the batch input data
	var batchInput []jobs.BatchInput_t

	// Loop through each record in the CSV file
	for i, record := range records {
		// Check if the record has exactly 3 fields: ID, Type, and Amount
		if len(record) != 3 {
			// If the record does not have 3 fields, return false indicating the file is invalid
			return false, jobs.JSONstr{}, nil, "", "", ""
		}

		// Parse the Amount field from the third element of the record
		amount, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			// If parsing the amount fails, return false indicating the file is invalid
			return false, jobs.JSONstr{}, nil, "", "", ""
		}

		// Create a Transaction struct with the parsed data
		transaction := Transaction{
			ID:     record[0], // The first field is the transaction ID
			Type:   record[1], // The second field is the transaction type
			Amount: amount,    // The parsed amount
		}

		// Convert the transaction to a JSON string
		jsonStr, err := jobs.NewJSONstr(fmt.Sprintf(
			`{"id": "%s", "type": "%s", "amount": %.2f}`,
			transaction.ID, transaction.Type, transaction.Amount,
		))
		if err != nil {
			// If converting to JSON fails, return false indicating the file is invalid
			return false, jobs.JSONstr{}, nil, "", "", ""
		}

		// Append the transaction to the batch input slice
		batchInput = append(batchInput, jobs.BatchInput_t{
			Line:  i + 1,   // The line number in the file (starting from 1)
			Input: jsonStr, // The transaction data as a JSON string
		})
	}

	// Create a context JSON string containing the filename
	context, _ := jobs.NewJSONstr(fmt.Sprintf(`{"filename": "%s"}`, fileName))

	// Return true indicating the file is valid,
	// along with the context, batch input, application name, and operation name
	return true, context, batchInput, "bankapp", "processtransactions", ""
}
