package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/examples"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
)

const (
	ErrMsgIDInvalidJSON      = 1000
	ErrMsgIDInvalidTransType = 1001

	ErrCodeInvalidJSON      = "invalid_json"
	ErrCodeInvalidTransType = "invalid_transaction_type"
)

type TransactionInput struct {
	TransactionID string  `json:"transactionID"`
	Type          string  `json:"type"`
	Amount        float64 `json:"amount"`
}

type TransactionBatchProcessor struct {
}

func (p *TransactionBatchProcessor) MarkDone(initBlock jobs.InitBlock, contextJson jobs.JSONstr, details jobs.BatchDetails_t) error {
	var contextData struct {
		NotificationEmail string `json:"notification_email"`
		BatchName         string `json:"batch_name"`
		Department        string `json:"department"`
		Priority          int    `json:"priority"`
		Filename          string `json:"filename"`
		BalanceKey        string `json:"balance_key"`
	}

	if err := json.Unmarshal([]byte(contextJson.String()), &contextData); err != nil {
		return fmt.Errorf("failed to parse context in MarkDone: %v", err)
	}

	log.Printf("\n[MarkDone] Batch %s completed", details.ID)
	log.Printf("  Success: %d, Failed: %d, Aborted: %d", details.NSuccess, details.NFailed, details.NAborted)

	return nil
}

func (p *TransactionBatchProcessor) DoBatchJob(initBlock jobs.InitBlock, batchctx jobs.JSONstr, line int, input jobs.JSONstr) (status batchsqlc.StatusEnum, result jobs.JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error) {
	var txInput TransactionInput
	err = json.Unmarshal([]byte(input.String()), &txInput)
	if err != nil {
		result, _ := jobs.NewJSONstr("")
		errMsg := wscutils.ErrorMessage{
			MsgID:   ErrMsgIDInvalidJSON,
			ErrCode: ErrCodeInvalidJSON,
		}
		messages = append(messages, errMsg)
		return batchsqlc.StatusEnumFailed, result, messages, nil, err
	}

	if txInput.Type != "DEPOSIT" && txInput.Type != "WITHDRAWAL" {
		result, _ := jobs.NewJSONstr("")
		errMsg := wscutils.ErrorMessage{
			MsgID:   ErrMsgIDInvalidTransType,
			ErrCode: ErrCodeInvalidTransType,
			Field:   "type",
			Vals:    []string{txInput.Type},
		}
		messages = append(messages, errMsg)
		return batchsqlc.StatusEnumFailed, result, messages, nil, fmt.Errorf("invalid transaction type: %s", txInput.Type)
	}

	var batchCtx struct {
		Filename   string `json:"filename"`
		BalanceKey string `json:"balance_key"`
	}
	err = json.Unmarshal([]byte(batchctx.String()), &batchCtx)
	if err != nil {
		result, _ := jobs.NewJSONstr("")
		return batchsqlc.StatusEnumFailed, result, nil, nil, fmt.Errorf("failed to parse batchctx: %v", err)
	}

	redisClient := initBlock.(*TransactionInitBlock).RedisClient
	balanceKey := batchCtx.BalanceKey

	// Use atomic INCRBYFLOAT - no retries needed
	amount := txInput.Amount
	if txInput.Type == "WITHDRAWAL" {
		amount = -amount
	}

	err = redisClient.IncrByFloat(context.Background(), balanceKey, amount).Err()
	if err != nil {
		result, _ := jobs.NewJSONstr("")
		return batchsqlc.StatusEnumFailed, result, nil, nil, fmt.Errorf("failed to update balance: %v", err)
	}

	result, _ = jobs.NewJSONstr(`{"message": "Transaction processed successfully"}`)
	return batchsqlc.StatusEnumSuccess, result, nil, nil, nil
}

type TransactionInitializer struct{}

func (i *TransactionInitializer) Init(app string) (jobs.InitBlock, error) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	lctx := logharbour.LoggerContext{}
	logger := logharbour.NewLogger(&lctx, "TransactionBatchProcessor", os.Stdout)

	batchid := rand.Intn(100000)

	return &TransactionInitBlock{RedisClient: redisClient, Logger: logger, BatchID: batchid}, nil
}

type TransactionInitBlock struct {
	RedisClient *redis.Client
	Logger      *logharbour.Logger
	BatchID     int
}

func (ib *TransactionInitBlock) Close() error {
	return ib.RedisClient.Close()
}

func loadBatchInputFromCSV(csvFile string, startLine, count int) ([]jobs.BatchInput_t, error) {
	file, err := os.Open(csvFile)
	if err != nil {
		return nil, fmt.Errorf("error opening CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV records: %v", err)
	}

	endLine := startLine + count
	if endLine > len(records) {
		endLine = len(records)
	}

	var batchInput []jobs.BatchInput_t
	for i := startLine; i < endLine; i++ {
		record := records[i]
		if len(record) != 3 {
			return nil, fmt.Errorf("invalid CSV record at line %d", i+1)
		}

		amount, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid amount at line %d: %v", i+1, err)
		}

		txInput := TransactionInput{
			TransactionID: record[0],
			Type:          record[1],
			Amount:        amount,
		}

		txInputBytes, err := json.Marshal(txInput)
		if err != nil {
			return nil, fmt.Errorf("error marshalling transaction input at line %d: %v", i+1, err)
		}

		input, err := jobs.NewJSONstr(string(txInputBytes))
		if err != nil {
			return nil, fmt.Errorf("error creating JSONstr for transaction input at line %d: %v", i+1, err)
		}
		batchInput = append(batchInput, jobs.BatchInput_t{
			Line:  i + 1,
			Input: input,
		})
	}

	return batchInput, nil
}

func getDb() *pgxpool.Pool {
	connString := "postgres://alyatest:alyatest@localhost:5432/alyatest?sslmode=disable"
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	return pool
}

func main() {
	numWorkers := 10
	numBatches := 10
	rowsPerBatch := 10000

	if len(os.Args) > 1 {
		var err error
		numWorkers, err = strconv.Atoi(os.Args[1])
		if err != nil {
			log.Fatal("Invalid number of workers:", err)
		}
	}
	if len(os.Args) > 2 {
		var err error
		numBatches, err = strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatal("Invalid number of batches:", err)
		}
	}

	log.Printf("=== Multi-Batch Concurrent Processing Test ===\n")
	log.Printf("Workers: %d, Batches: %d, Rows per batch: %d\n", numWorkers, numBatches, rowsPerBatch)
	log.Printf("Total transactions: %d\n\n", numBatches*rowsPerBatch)

	pool := getDb()

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	minioClient := examples.CreateMinioClient()

	lctx := logharbour.LoggerContext{}
	logger := logharbour.NewLogger(&lctx, "JobManager", os.Stdout)

	jm := jobs.NewJobManager(pool, redisClient, minioClient, logger, &jobs.JobManagerConfig{
		BatchOutputBucket: "batch-output",
	})

	err := jm.RegisterProcessorBatch("transactionapp", "processtransactions", &TransactionBatchProcessor{})
	if err != nil {
		log.Fatal("Failed to register batch processor:", err)
	}
	err = jm.RegisterInitializer("transactionapp", &TransactionInitializer{})
	if err != nil {
		log.Fatal("Failed to register initializer:", err)
	}

	csvFile := "generate_txn/transactions.csv"

	filename := "transactions.csv"
	balanceKey := fmt.Sprintf("batch:%s:balance", filename)
	err = redisClient.Set(context.Background(), balanceKey, 0, 0).Err()
	if err != nil {
		log.Fatal("Failed to set initial balance in Redis:", err)
	}

	var batchIDs []string

	log.Printf("Creating %d batches...\n", numBatches)
	for i := 0; i < numBatches; i++ {
		startLine := i * rowsPerBatch
		batchInput, err := loadBatchInputFromCSV(csvFile, startLine, rowsPerBatch)
		if err != nil {
			log.Fatalf("Failed to load batch input from CSV (batch %d): %v", i+1, err)
		}

		contextData := map[string]interface{}{
			"notification_email": "supervisor@example.com",
			"batch_name":         fmt.Sprintf("Transaction Batch %d", i+1),
			"department":         "Finance",
			"priority":           1,
			"filename":           filename,
			"balance_key":        balanceKey,
		}

		contextJSON, err := json.Marshal(contextData)
		if err != nil {
			log.Fatalf("Failed to create context JSON for batch %d: %v", i+1, err)
		}

		batchContext, err := jobs.NewJSONstr(string(contextJSON))
		if err != nil {
			log.Fatalf("Failed to create batch context for batch %d: %v", i+1, err)
		}

		batchID, err := jm.BatchSubmit(
			"transactionapp",
			"processtransactions",
			batchContext,
			batchInput,
			false,
		)
		if err != nil {
			log.Fatalf("Failed to submit batch %d: %v", i+1, err)
		}

		batchIDs = append(batchIDs, batchID)
		log.Printf("  Batch %d submitted: %s (rows %d-%d)\n", i+1, batchID, startLine+1, startLine+rowsPerBatch)
	}

	log.Printf("\nStarting %d concurrent workers...\n\n", numWorkers)

	var wg sync.WaitGroup
	startTime := time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		workerID := i + 1
		go func(id int) {
			defer wg.Done()
			log.Printf("[Worker %d] Started\n", id)
			jm.RunWithContext(ctx)
			log.Printf("[Worker %d] Stopped\n", id)
		}(workerID)
	}

	log.Printf("Polling for batch completion...\n")

	completedBatches := make(map[string]bool)
	lastUpdate := time.Now()

	for {
		allCompleted := true
		totalSuccess := 0
		totalFailed := 0
		totalAborted := 0

		for _, batchID := range batchIDs {
			if completedBatches[batchID] {
				continue
			}

			status, _, _, nsuccess, nfailed, naborted, err := jm.BatchDone(batchID)
			if err != nil {
				log.Printf("Error polling batch %s: %v\n", batchID, err)
				continue
			}

			totalSuccess += nsuccess
			totalFailed += nfailed
			totalAborted += naborted

			if status == batchsqlc.StatusEnumQueued || status == batchsqlc.StatusEnumInprog {
				allCompleted = false
			} else {
				if !completedBatches[batchID] {
					completedBatches[batchID] = true
					log.Printf("[Batch Complete] %s - Status: %s (success: %d, failed: %d)\n",
						batchID, status, nsuccess, nfailed)
				}
			}
		}

		if time.Since(lastUpdate) > 2*time.Second {
			log.Printf("Progress: %d/%d batches completed (total processed: %d)\n",
				len(completedBatches), numBatches, totalSuccess+totalFailed+totalAborted)
			lastUpdate = time.Now()
		}

		if allCompleted {
			duration := time.Since(startTime)
			log.Printf("\n=== All Batches Completed ===\n")
			log.Printf("Total Success: %d\n", totalSuccess)
			log.Printf("Total Failed: %d\n", totalFailed)
			log.Printf("Total Aborted: %d\n", totalAborted)
			log.Printf("Total Duration: %v\n", duration)
			log.Printf("Throughput: %.2f transactions/second\n", float64(totalSuccess+totalFailed+totalAborted)/duration.Seconds())
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	finalBalance, err := redisClient.Get(context.Background(), balanceKey).Float64()
	if err != nil {
		log.Printf("Error getting final balance: %v\n", err)
	} else {
		log.Printf("\nFinal Balance in Redis: %.2f\n", finalBalance)
	}

	log.Printf("\nStopping workers via context cancellation...\n")
	cancel()
	wg.Wait()

	log.Printf("\nVerifying batch summarization in database...\n")
	for i, batchID := range batchIDs {
		row := pool.QueryRow(context.Background(),
			"SELECT status, nsuccess, nfailed, naborted FROM batches WHERE id = $1", batchID)
		var status string
		var nsuccess, nfailed, naborted int
		err := row.Scan(&status, &nsuccess, &nfailed, &naborted)
		if err != nil {
			log.Printf("Batch %d (%s): Error reading: %v\n", i+1, batchID, err)
		} else {
			log.Printf("Batch %d: status=%s, success=%d, failed=%d, aborted=%d\n",
				i+1, status, nsuccess, nfailed, naborted)
		}
	}

	log.Printf("\nTest completed successfully!\n")
}
