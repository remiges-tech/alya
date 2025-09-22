package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/remiges-tech/alya/jobs"
)

// checkBankTransactionFile processes a CSV file of bank transactions.
// Validates records and converts them to JSON for batch processing.
func checkBankTransactionFile(fileContents string, fileName string, batchctx jobs.JSONstr) (bool, jobs.JSONstr, []jobs.BatchInput_t, string, string, string) {
	// Log received context
	log.Printf("FileChk received input context: %s", batchctx.String())

	reader := csv.NewReader(strings.NewReader(fileContents))
	records, err := reader.ReadAll()
	if err != nil {
		log.Printf("Failed to read CSV: %v", err)
		return false, jobs.JSONstr{}, nil, "", "", ""
	}

	var batchInput []jobs.BatchInput_t
	for i, record := range records {
		if len(record) != 3 {
			log.Printf("Invalid record format at line %d: expected 3 fields, got %d", i+1, len(record))
			return false, jobs.JSONstr{}, nil, "", "", ""
		}

		amount, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			log.Printf("Invalid amount at line %d: %v", i+1, err)
			return false, jobs.JSONstr{}, nil, "", "", ""
		}

		// Validate amount is positive
		if amount <= 0 {
			log.Printf("Invalid amount at line %d: amount must be positive, got %.2f", i+1, amount)
			return false, jobs.JSONstr{}, nil, "", "", ""
		}

		transaction := Transaction{
			ID:     record[0],
			Type:   record[1],
			Amount: amount,
		}

		jsonStr, err := jobs.NewJSONstr(fmt.Sprintf(`{"id": "%s", "type": "%s", "amount": %.2f}`, transaction.ID, transaction.Type, transaction.Amount))
		if err != nil {
			log.Printf("Failed to create JSON for transaction at line %d: %v", i+1, err)
			return false, jobs.JSONstr{}, nil, "", "", ""
		}

		batchInput = append(batchInput, jobs.BatchInput_t{
			Line:  i + 1,
			Input: jsonStr,
		})
	}

	// Merge input context with file data
	var inputCtx map[string]interface{}
	if err := json.Unmarshal([]byte(batchctx.String()), &inputCtx); err != nil {
		log.Printf("Failed to parse input context: %v", err)
		inputCtx = make(map[string]interface{})
	}

	// Add file metadata
	inputCtx["filename"] = fileName
	inputCtx["record_count"] = len(records)
	inputCtx["validated_by"] = "checkBankTransactionFile"
	inputCtx["file_size_bytes"] = len(fileContents)

	// Create merged context
	mergedJSON, err := json.Marshal(inputCtx)
	if err != nil {
		log.Printf("Failed to marshal merged context: %v", err)
		batchContext, _ := jobs.NewJSONstr(`{"filename": "` + fileName + `", "error": "context_merge_failed"}`)
		return true, batchContext, batchInput, "bankapp", "processtransactions", ""
	}

	batchContext, _ := jobs.NewJSONstr(string(mergedJSON))
	log.Printf("FileChk returning merged context: %s", batchContext.String())

	return true, batchContext, batchInput, "bankapp", "processtransactions", ""
}
