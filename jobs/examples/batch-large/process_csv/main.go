package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/examples"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

type TransactionInput struct {
	TransactionID string  `json:"transactionID"`
	Type          string  `json:"type"`
	Amount        float64 `json:"amount"`
}

type TransactionBatchProcessor struct{}

func (p *TransactionBatchProcessor) DoBatchJob(initBlock jobs.InitBlock, batchctx jobs.JSONstr, line int, input jobs.JSONstr) (status batchsqlc.StatusEnum, result jobs.JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error) {
	// Parse the input JSON
	var txInput TransactionInput
	err = json.Unmarshal([]byte(input), &txInput)
	if err != nil {
		return batchsqlc.StatusEnumFailed, "", nil, nil, err
	}

	// Simulate processing the transaction
	fmt.Printf("Processing transaction %s of type %s with amount %.2f\n", txInput.TransactionID, txInput.Type, txInput.Amount)
	time.Sleep(time.Second) // Simulating processing delay

	// Update the balance in Redis based on the transaction type
	redisClient := initBlock.(*TransactionInitBlock).RedisClient
	balanceKey := "batch:balance"

	// Perform the balance update operation in Redis
	err = redisClient.Watch(context.Background(), func(tx *redis.Tx) error {
		balance, err := tx.Get(context.Background(), balanceKey).Float64()
		if err != nil && err != redis.Nil {
			return err
		}

		if txInput.Type == "DEPOSIT" {
			balance += txInput.Amount
		} else if txInput.Type == "WITHDRAWAL" {
			balance -= txInput.Amount
		}

		_, err = tx.TxPipelined(context.Background(), func(pipe redis.Pipeliner) error {
			pipe.Set(context.Background(), balanceKey, balance, 0)
			return nil
		})
		return err
	})
	if err != nil {
		return batchsqlc.StatusEnumFailed, "", nil, nil, fmt.Errorf("failed to update balance in Redis: %v", err)
	}

	// Generate blobRows data
	blobRows = map[string]string{
		"transaction_summary.txt": fmt.Sprintf("%s,%.2f\n", txInput.TransactionID, txInput.Amount),
	}

	// Return success status
	return batchsqlc.StatusEnumSuccess, jobs.JSONstr(`{"message": "Transaction processed successfully"}`), nil, blobRows, nil
}

type TransactionInitializer struct{}

func (i *TransactionInitializer) Init(app string) (jobs.InitBlock, error) {
	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	return &TransactionInitBlock{RedisClient: redisClient}, nil
}

type TransactionInitBlock struct {
	RedisClient *redis.Client
}

func (ib *TransactionInitBlock) Close() error {
	// Close the Redis client connection
	return ib.RedisClient.Close()
}

func loadBatchInputFromCSV(csvFile string) ([]batchsqlc.InsertIntoBatchRowsParams, error) {
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

	var batchInput []batchsqlc.InsertIntoBatchRowsParams
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

		batchInput = append(batchInput, batchsqlc.InsertIntoBatchRowsParams{
			Line:  int32(i + 1),
			Input: txInputBytes,
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

	// Initialize JobManager
	jm := jobs.NewJobManager(pool, redisClient, minioClient)

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
	csvFile := "transactions.csv"
	batchInput, err := loadBatchInputFromCSV(csvFile)
	if err != nil {
		log.Fatal("Failed to load batch input from CSV:", err)
	}

	// Submit the batch
	batchID, err := jm.BatchSubmit("transactionapp", "processtransactions", jobs.JSONstr("{}"), batchInput, false)
	if err != nil {
		log.Fatal("Failed to submit batch:", err)
	}
	fmt.Println("Batch submitted. Batch ID:", batchID)

	// Start the JobManager in a separate goroutine
	go jm.Run()

	// Poll for the batch completion status
	for {
		status, _, outputFiles, nsuccess, nfailed, naborted, err := jm.BatchDone(batchID)
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
