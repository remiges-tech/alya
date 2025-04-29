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

// Add MarkDone method
func (p *TransactionBatchProcessor) MarkDone(initBlock jobs.InitBlock, contextJson jobs.JSONstr, details jobs.BatchDetails_t) error {
	// Parse the context to get our configuration
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

	log.Printf("\nMarkDone: Processing completion for batch %s", details.ID)
	log.Printf("Batch Context Details:")
	log.Printf("- Notification Email: %s", contextData.NotificationEmail)
	log.Printf("- Batch Name: %s", contextData.BatchName)
	log.Printf("- Department: %s", contextData.Department)
	log.Printf("- Priority: %d", contextData.Priority)

	// Get and print the final balance
	if contextData.BalanceKey != "" {
		// Access Redis client from main program context
		redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
		// Use standard context package for Redis operations
		ctx := context.Background()
		balance, err := redisClient.Get(ctx, contextData.BalanceKey).Result()
		if err != nil && err != redis.Nil {
			log.Printf("Error getting final balance: %v", err)
		} else {
			log.Printf("\nFinal Redis Balance Key: %s", contextData.BalanceKey)
			log.Printf("Final Balance Value: %s", balance)
		}
		// Close the Redis connection
		redisClient.Close()
	}

	log.Printf("\nBatch Statistics:")
	log.Printf("- Successfully processed: %d", details.NSuccess)
	log.Printf("- Failed: %d", details.NFailed)
	log.Printf("- Aborted: %d", details.NAborted)

	// Simulate sending notification email using context data
	notificationBody := fmt.Sprintf(`
		Batch Processing Complete
		------------------------
		Batch Name: %s
		Department: %s
		Priority: %d
		
		Results:
		- Success: %d
		- Failed: %d
		- Aborted: %d
		
		Status: %s
		
		Output Files:
	`, contextData.BatchName, contextData.Department, contextData.Priority,
		details.NSuccess, details.NFailed, details.NAborted, details.Status)

	for filename, objectID := range details.OutputFiles {
		notificationBody += fmt.Sprintf("\n- %s: %s", filename, objectID)
	}

	log.Printf("\nSending notification email to: %s", contextData.NotificationEmail)
	log.Printf("Email content:\n%s", notificationBody)

	return nil
}

func (p *TransactionBatchProcessor) DoBatchJob(initBlock jobs.InitBlock, batchctx jobs.JSONstr, line int, input jobs.JSONstr) (status batchsqlc.StatusEnum, result jobs.JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error) {
	logger := initBlock.(*TransactionInitBlock).Logger
	// Parse the input JSON
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

	// Validate transaction type
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

	// Parse the batchctx JSON to get the filename
	var batchCtx struct {
		Filename   string `json:"filename"`
		BalanceKey string `json:"balance_key"`
	}
	err = json.Unmarshal([]byte(batchctx.String()), &batchCtx)
	if err != nil {
		result, _ := jobs.NewJSONstr("")
		return batchsqlc.StatusEnumFailed, result, nil, nil, fmt.Errorf("failed to parse batchctx: %v", err)
	}

	// Simulate processing the transaction
	log := fmt.Sprintf("Processing transaction %s of type %s with amount %.2f from file %s", txInput.TransactionID, txInput.Type, txInput.Amount, batchCtx.Filename)
	logger.Log(log)
	// time.Sleep(time.Second) // Simulating processing delay

	// Update the balance in Redis based on the transaction type
	redisClient := initBlock.(*TransactionInitBlock).RedisClient
	balanceKey := batchCtx.BalanceKey

	const maxRetries = 50

	for retry := 0; retry < maxRetries; retry++ {
		err = redisClient.Watch(context.Background(), func(tx *redis.Tx) error {
			balance, err := tx.Get(context.Background(), balanceKey).Float64()
			if err != nil && err != redis.Nil {
				return err
			}

			var newBalance float64
			if txInput.Type == "DEPOSIT" {
				newBalance = balance + txInput.Amount
			} else if txInput.Type == "WITHDRAWAL" {
				newBalance = balance - txInput.Amount
			}

			_, err = tx.TxPipelined(context.Background(), func(pipe redis.Pipeliner) error {
				pipe.Set(context.Background(), balanceKey, newBalance, 0)
				return nil
			})
			return err
		}, balanceKey)

		if err == nil {
			// Transaction succeeded, break the loop
			break
		}

		if err == redis.TxFailedErr {
			// Transaction failed due to key being updated, retry
			continue
		}

		// Handle other errors
		result, _ := jobs.NewJSONstr("")
		return batchsqlc.StatusEnumFailed, result, nil, nil, fmt.Errorf("failed to update balance in Redis: %v", err)
	}

	if err != nil {
		// Maximum retries exceeded
		log := fmt.Sprintf("Failed to update balance in Redis for txn %s after %d retries: %v", txInput.TransactionID, maxRetries, err)
		logger.LogActivity(log, "")
		result, _ := jobs.NewJSONstr("")
		return batchsqlc.StatusEnumFailed, result, nil, nil, fmt.Errorf("failed to update balance in Redis for txn %s after %d retries: %v", txInput.TransactionID, maxRetries, err)
	}

	// Generate blobRows data
	blobRows = map[string]string{
		"transaction_summary.txt": fmt.Sprintf("%s,%.2f\n", txInput.TransactionID, txInput.Amount),
	}

	// Return success status
	result, _ = jobs.NewJSONstr(`{"message": "Transaction processed successfully"}`)
	return batchsqlc.StatusEnumSuccess, result, nil, blobRows, nil
}

type TransactionInitializer struct{}

func (i *TransactionInitializer) Init(app string) (jobs.InitBlock, error) {
	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Create a new logger that writes to stdout
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
	// Close the Redis client connection
	return ib.RedisClient.Close()
}

func loadBatchInputFromCSV(csvFile string) ([]jobs.BatchInput_t, error) {
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

	var batchInput []jobs.BatchInput_t
	for i, record := range records {
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

func main() {
	// Initialize the database connection
	pool := getDb()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Create a new Minio client instance with the default credentials
	minioClient := examples.CreateMinioClient()

	// Create a new logger that writes to stdout
	lctx := logharbour.LoggerContext{}
	logger := logharbour.NewLogger(&lctx, "JobManager", os.Stdout)

	// Initialize JobManager
	jm := jobs.NewJobManager(pool, redisClient, minioClient, logger, &jobs.JobManagerConfig{
		BatchOutputBucket: "batch-output",
	})

	// Register the batch processor and initializer
	err := jm.RegisterProcessorBatch("transactionapp", "processtransactions", &TransactionBatchProcessor{})
	if err != nil {
		log.Fatal("Failed to register batch processor:", err)
	}
	err = jm.RegisterInitializer("transactionapp", &TransactionInitializer{})
	if err != nil {
		log.Fatal("Failed to register initializer:", err)
	}

	// Load batch input from CSV file
	csvFile := "../generate_txn/transactions.csv"
	batchInput, err := loadBatchInputFromCSV(csvFile)
	if err != nil {
		log.Fatal("Failed to load batch input from CSV:", err)
	}

	// Prepare the batch context with the filename
	filename := "transactions.csv"
	balanceKey := fmt.Sprintf("batch:%s:balance", filename)
	err = redisClient.Set(context.Background(), balanceKey, 0, 0).Err()
	if err != nil {
		log.Fatal("Failed to set initial balance in Redis:", err)
	}

	// Create context with configuration for the batch
	contextData := map[string]interface{}{
		"notification_email": "supervisor@example.com",
		"batch_name":         "Daily Transaction Processing",
		"department":         "Finance",
		"priority":           1,
		"filename":           filename,   // Add the filename to context to use in DoBatchJob
		"balance_key":        balanceKey, // Store the balance key in context for MarkDone
	}

	contextJSON, err := json.Marshal(contextData)
	if err != nil {
		log.Fatal("Failed to create context JSON:", err)
	}

	batchContext, err := jobs.NewJSONstr(string(contextJSON))
	if err != nil {
		log.Fatal("Failed to create batch context:", err)
	}

	// Log the context before submitting the batch
	log.Printf("\nSubmitting batch with context:")
	log.Printf("- Notification Email: %s", contextData["notification_email"])
	log.Printf("- Batch Name: %s", contextData["batch_name"])
	log.Printf("- Department: %s", contextData["department"])
	log.Printf("- Priority: %d", contextData["priority"])
	log.Printf("- Filename: %s", contextData["filename"])

	// Submit the batch with our context
	batchID, err := jm.BatchSubmit(
		"transactionapp",
		"processtransactions",
		batchContext,
		batchInput,
		false,
	)
	if err != nil {
		log.Fatal("Failed to submit batch:", err)
	}

	fmt.Println("Batch submitted. Batch ID:", batchID)

	// Start the JobManager in a separate goroutine
	go jm.Run()

	// Poll for the batch completion status
	for {
		status, _, outputFiles, nsuccess, nfailed, naborted, err := jm.BatchDone(batchID)
		fmt.Printf("batchid: %v\n", batchID)
		fmt.Printf("status: %v\n", status)
		if err != nil {
			log.Fatal("Error while polling for batch status:", err)
		}

		if status == batchsqlc.StatusEnumQueued || status == batchsqlc.StatusEnumInprog {
			fmt.Println("Batch processing in progress. Trying again in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}

		fmt.Println("Batch completed with status:", status)
		fmt.Println("Output files:", outputFiles)
		fmt.Println("Success count:", nsuccess)
		fmt.Println("Failed count:", nfailed)
		fmt.Println("Aborted count:", naborted)
		break
	}

	// Wait a bit to see the MarkDone output
	fmt.Println("\nWaiting for batch to complete and MarkDone to be called...")
	time.Sleep(time.Second * 30)
}

func getDb() *pgxpool.Pool {
	// Configure the database connection details
	dbHost := "localhost"
	dbPort := 5432
	dbUser := "alyatest"
	dbPassword := "alyatest"
	dbName := "alyatest"

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatal("Error connecting to the database:", err)
	}

	return pool
}
