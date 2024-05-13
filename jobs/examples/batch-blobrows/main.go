package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
)

type TransactionInput struct {
	TransactionID string `json:"transactionID"`
	Type          string `json:"type"`
	Amount        string `json:"amount"`
}

type TransactionBatchProcessor struct{}

func (p *TransactionBatchProcessor) DoBatchJob(initBlock jobs.InitBlock, context jobs.JSONstr, line int, input jobs.JSONstr) (status batchsqlc.StatusEnum, result jobs.JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error) {
	// Parse the input JSON
	var txInput TransactionInput
	err = json.Unmarshal([]byte(input.String()), &txInput)
	if err != nil {
		emptyJson, _ := jobs.NewJSONstr("{}")
		return batchsqlc.StatusEnumFailed, emptyJson, nil, nil, err
	}

	// Simulate processing the transaction
	fmt.Printf("Processing transaction %s of type %s with amount %s\n", txInput.TransactionID, txInput.Type, txInput.Amount)
	time.Sleep(time.Second) // Simulating processing delay

	// Generate blobRows data for every alternate line
	if line%2 == 0 {
		blobRows = map[string]string{
			"transaction_summary.txt": fmt.Sprintf("%s,%s,%s\n", txInput.TransactionID, txInput.Type, txInput.Amount),
		}
	}

	// Return success status
	result, _ = jobs.NewJSONstr(`{"message": "Transaction processed successfully"}`)
	return batchsqlc.StatusEnumSuccess, result, nil, blobRows, nil
}

type TransactionInitializer struct{}

func (i *TransactionInitializer) Init(app string) (jobs.InitBlock, error) {
	// Initialize any necessary resources for transaction processing
	return &TransactionInitBlock{}, nil
}

type TransactionInitBlock struct{}

func (ib *TransactionInitBlock) Close() error {
	// Clean up any resources
	return nil
}

// This example demonstrates that not all input records necessarily result in output files. In `DoBatchJob`,
// output files are generated only for every alternate line. This illustrates a flexible handling
// of batch outputs where output generation is conditional.
func main() {
	// Initialize the database connection
	pool := getDb()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Create a new Minio client instance with the default credentials
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("Error creating Minio client: %v", err)
	}

	// Create the test bucket
	bucketName := "batch-output"
	err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
	if err != nil {
		// Check if the bucket already exists
		exists, err := minioClient.BucketExists(context.Background(), bucketName)
		if err != nil || !exists {
			log.Fatalf("Error creating test bucket: %v", err)
		}
	}

	lctx := logharbour.NewLoggerContext(logharbour.DefaultPriority)
	logger := logharbour.NewLogger(lctx, "JobManager", os.Stdout)

	// Initialize JobManager
	jm := jobs.NewJobManager(pool, redisClient, minioClient, logger, nil)

	// Register the batch processor and initializer
	err = jm.RegisterProcessorBatch("transactionapp", "processtransactions", &TransactionBatchProcessor{})
	if err != nil {
		log.Fatal("Failed to register batch processor:", err)
	}

	err = jm.RegisterInitializer("transactionapp", &TransactionInitializer{})
	if err != nil {
		log.Fatal("Failed to register initializer:", err)
	}

	input1, err := jobs.NewJSONstr(`{"transactionID": "TX001", "type": "DEPOSIT", "amount": "1000.00"}`)
	if err != nil {
		log.Fatal("Error creating JSONstr for transaction input at line 1:", err)
	}
	input2, err := jobs.NewJSONstr(`{"transactionID": "TX002", "type": "WITHDRAWAL", "amount": "500.00"}`)
	if err != nil {
		log.Fatal("Error creating JSONstr for transaction input at line 2:", err)
	}
	input3, err := jobs.NewJSONstr(`{"transactionID": "TX003", "type": "TRANSFER", "amount": "750.00"}`)
	if err != nil {
		log.Fatal("Error creating JSONstr for transaction input at line 3:", err)
	}
	input4, err := jobs.NewJSONstr(`{"transactionID": "TX004", "type": "PAYMENT", "amount": "250.00"}`)
	if err != nil {
		log.Fatal("Error creating JSONstr for transaction input at line 4:", err)
	}
	// Prepare the batch input data
	batchInput := []jobs.BatchInput_t{
		{
			Line:  1,
			Input: input1,
		},
		{
			Line:  2,
			Input: input2,
		},
		{
			Line:  3,
			Input: input3,
		},
		{
			Line:  4,
			Input: input4,
		},
	}

	// Submit the batch
	emptyJson, _ := jobs.NewJSONstr("{}")
	batchID, err := jm.BatchSubmit("transactionapp", "processtransactions", emptyJson, batchInput, false)
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

		// Access the output file from the object store
		if outputFile, ok := outputFiles["transaction_summary.txt"]; ok {
			object, err := minioClient.GetObject(context.Background(), bucketName, outputFile, minio.GetObjectOptions{})
			if err != nil {
				log.Fatal("Error retrieving output file from object store:", err)
			}
			defer object.Close()

			// Read the output file content
			outputFileContent, err := readAll(object)
			if err != nil {
				log.Fatal("Error reading output file content:", err)
			}
			fmt.Println("Output file content:")
			fmt.Println(string(outputFileContent))
		}

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

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)
	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatal("Error connecting to the database:", err)
	}
	return pool
}

// Read the entire content of a reader into a byte slice.
func readAll(r io.Reader) ([]byte, error) {
	var buf []byte
	for {
		b := make([]byte, 1024)
		n, err := r.Read(b)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}
		buf = append(buf, b[:n]...)
	}
	return buf, nil
}
