package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

type BankTransactionProcessor struct{}

type Transaction struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Amount float64 `json:"amount"`
}

func (p *BankTransactionProcessor) DoBatchJob(initBlock jobs.InitBlock, context jobs.JSONstr, line int, input jobs.JSONstr) (status batchsqlc.StatusEnum, result jobs.JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error) {
	// Log received context
	log.Printf("BatchProcessor (line %d) received context: %s", line, context.String())

	var transaction Transaction
	err = json.Unmarshal([]byte(input.String()), &transaction)
	if err != nil {
		return batchsqlc.StatusEnumFailed, jobs.JSONstr{}, nil, nil, fmt.Errorf("failed to unmarshal transaction: %v", err)
	}

	// Process the transaction (in a real-world scenario, you'd update account balances, etc.)
	resultStr := fmt.Sprintf("Processed transaction %s: %s %.2f", transaction.ID, transaction.Type, transaction.Amount)
	result, err = jobs.NewJSONstr(fmt.Sprintf(`{"result": "%s"}`, resultStr))
	if err != nil {
		return batchsqlc.StatusEnumFailed, jobs.JSONstr{}, nil, nil, fmt.Errorf("failed to create result JSON: %v", err)
	}

	return batchsqlc.StatusEnumSuccess, result, nil, nil, nil
}
