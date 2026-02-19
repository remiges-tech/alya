package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

type RowInput struct {
	RowNumber int `json:"row_number"`
}

type SlowProcessor struct{}

func (p *SlowProcessor) DoBatchJob(
	initBlock jobs.InitBlock,
	context jobs.JSONstr,
	line int,
	input jobs.JSONstr,
) (
	status batchsqlc.StatusEnum,
	result jobs.JSONstr,
	messages []wscutils.ErrorMessage,
	blobRows map[string]string,
	err error,
) {
	var rowInput RowInput
	err = json.Unmarshal([]byte(input.String()), &rowInput)
	if err != nil {
		emptyJSON, _ := jobs.NewJSONstr("{}")
		return batchsqlc.StatusEnumFailed, emptyJSON, nil, nil, err
	}

	hostname, _ := os.Hostname()
	pid := os.Getpid()

	fmt.Printf("[%s] Row %d: START processing (worker: %s-%d)\n",
		time.Now().Format("15:04:05"),
		rowInput.RowNumber,
		hostname,
		pid,
	)

	time.Sleep(processingTime)

	fmt.Printf("[%s] Row %d: DONE processing\n",
		time.Now().Format("15:04:05"),
		rowInput.RowNumber,
	)

	result, _ = jobs.NewJSONstr(fmt.Sprintf(`{"processed_by": "%s-%d", "row": %d}`,
		hostname, pid, rowInput.RowNumber))

	return batchsqlc.StatusEnumSuccess, result, nil, nil, nil
}

func (p *SlowProcessor) MarkDone(
	initBlock jobs.InitBlock,
	context jobs.JSONstr,
	details jobs.BatchDetails_t,
) error {
	fmt.Println()
	fmt.Println("=================================================")
	fmt.Println("Batch completed")
	fmt.Println("=================================================")
	fmt.Printf("Batch ID:    %s\n", details.ID)
	fmt.Printf("Status:      %s\n", details.Status)
	fmt.Printf("Successful:  %d\n", details.NSuccess)
	fmt.Printf("Failed:      %d\n", details.NFailed)
	fmt.Printf("Aborted:     %d\n", details.NAborted)
	return nil
}

type SlowInitializer struct{}

func (i *SlowInitializer) Init(app string) (jobs.InitBlock, error) {
	return &SlowInitBlock{}, nil
}

type SlowInitBlock struct{}

func (ib *SlowInitBlock) Close() error {
	return nil
}
