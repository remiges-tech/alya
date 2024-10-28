package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
)

type EmailInput struct {
	RecipientEmail string `json:"recipientEmail"`
	Subject        string `json:"subject"`
	Body           string `json:"body"`
}

type EmailBatchProcessor struct{}

func (p *EmailBatchProcessor) DoBatchJob(initBlock jobs.InitBlock, context jobs.JSONstr, line int, input jobs.JSONstr) (status batchsqlc.StatusEnum, result jobs.JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error) {
	// Parse the input JSON
	var emailInput EmailInput
	err = json.Unmarshal([]byte(input.String()), &emailInput)
	if err != nil {
		emptyJson, _ := jobs.NewJSONstr("{}")
		return batchsqlc.StatusEnumFailed, emptyJson, nil, nil, err
	}

	// Simulate sending the email
	fmt.Printf("Sending email to %s with subject: %s\n", emailInput.RecipientEmail, emailInput.Subject)
	time.Sleep(time.Second) // Simulating email sending delay

	// Generate sample blobRows data
	blobRows = map[string]string{
		"emailLog": fmt.Sprintf("Email sent to %s with subject: %s", emailInput.RecipientEmail, emailInput.Subject),
	}

	// Return success status
	result, _ = jobs.NewJSONstr(`{"message": "Email sent successfully"}`)
	return batchsqlc.StatusEnumSuccess, result, nil, blobRows, nil
}

// Add MarkDone method to EmailBatchProcessor
func (p *EmailBatchProcessor) MarkDone(initBlock jobs.InitBlock, context jobs.JSONstr, details jobs.BatchDetails_t) error {
	log.Printf("Email batch %s completed with status: %s", details.ID, details.Status)
	log.Printf("Results: Success=%d, Failed=%d, Aborted=%d",
		details.NSuccess, details.NFailed, details.NAborted)
	return nil
}

type EmailInitializer struct{}

func (i *EmailInitializer) Init(app string) (jobs.InitBlock, error) {
	// Initialize any necessary resources for email sending
	return &EmailInitBlock{}, nil
}

type EmailInitBlock struct{}

func (ib *EmailInitBlock) Close() error {
	// Clean up any resources
	return nil
}

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
	err = jm.RegisterProcessorBatch("emailapp", "sendbulkemail", &EmailBatchProcessor{})
	if err != nil {
		log.Fatal("Failed to register batch processor:", err)
	}

	err = jm.RegisterInitializer("emailapp", &EmailInitializer{})
	if err != nil {
		log.Fatal("Failed to register initializer:", err)
	}

	// Submit the initial batch
	batchID := submitBatch(jm)

	// Prepare additional batch input data
	additionalBatchInput := []batchsqlc.InsertIntoBatchRowsParams{
		{
			Batch: batchID,
			Line:  3,
			Input: []byte(`{"recipientEmail": "user3@example.com", "subject": "Batch Email 3", "body": "Hello, this is batch email 3."}`),
		},
		{
			Batch: batchID,
			Line:  4,
			Input: []byte(`{"recipientEmail": "user4@example.com", "subject": "Batch Email 4", "body": "Hello, this is batch email 4."}`),
		},
		// Add more batch input records as needed
	}

	// Start the JobManager in a separate goroutine
	fmt.Println("Starting the JobManager...")
	go jm.Run()

	// sleep for 1 minute
	fmt.Println("Sleeping for 1 minute...")
	time.Sleep(1 * time.Minute)
	// Append the additional batch input to the existing batch
	nrows, err := jm.BatchAppend(batchID.String(), additionalBatchInput, false)
	if err != nil {
		log.Fatal("Failed to append to batch:", err)
	}

	fmt.Println("Total rows in the batch:", nrows)

	// Poll for the batch completion status
	for {
		status, batchOutput, outputFiles, nsuccess, nfailed, naborted, err := jm.BatchDone(batchID.String())
		if err != nil {
			log.Fatal("Error while polling for batch status:", err)
		}

		if status == batchsqlc.StatusEnumQueued || status == batchsqlc.StatusEnumInprog {
			fmt.Println("Batch processing in progress. Trying again in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}

		fmt.Println("Batch completed with status:", status)
		fmt.Println("Batch output:", batchOutput)
		fmt.Println("Output files:", outputFiles)
		fmt.Println("Success count:", nsuccess)
		fmt.Println("Failed count:", nfailed)
		fmt.Println("Aborted count:", naborted)
		break
	}
}

func submitBatch(jm *jobs.JobManager) uuid.UUID {

	input1, err := jobs.NewJSONstr(`{"recipientEmail": "user1@example.com", "subject": "Batch Email 1", "body": "Hello, this is batch email 1."}`)
	if err != nil {
		log.Fatal("Error creating JSONstr for transaction input at line 1:", err)
	}
	input2, err := jobs.NewJSONstr(`{"recipientEmail": "user2@example.com", "subject": "Batch Email 2", "body": "Hello, this is batch email 2."}`)
	if err != nil {
		log.Fatal("Error creating JSONstr for transaction input at line 2:", err)
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
	}

	// Submit the batch
	waitabit := true
	emptyJson, _ := jobs.NewJSONstr("{}")
	batchID, err := jm.BatchSubmit("emailapp", "sendbulkemail", emptyJson, batchInput, waitabit)
	if err != nil {
		log.Fatal("Failed to submit batch:", err)
	}

	fmt.Println("Batch submitted. Batch ID:", batchID)

	return uuid.MustParse(batchID)
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
