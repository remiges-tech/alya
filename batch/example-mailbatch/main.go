package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/batch"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

type EmailInput struct {
	RecipientEmail string `json:"recipientEmail"`
	Subject        string `json:"subject"`
	Body           string `json:"body"`
}

type EmailBatchProcessor struct{}

func (p *EmailBatchProcessor) DoBatchJob(initBlock batch.InitBlock, context batch.JSONstr, line int, input batch.JSONstr) (status batchsqlc.StatusEnum, result batch.JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error) {
	// Parse the input JSON
	var emailInput EmailInput
	err = json.Unmarshal([]byte(input), &emailInput)
	if err != nil {
		return batchsqlc.StatusEnumFailed, "", nil, nil, err
	}

	// Simulate sending the email
	fmt.Printf("Sending email to %s with subject: %s\n", emailInput.RecipientEmail, emailInput.Subject)
	time.Sleep(time.Second) // Simulating email sending delay

	// Generate sample blobRows data
	blobRows = map[string]string{
		"emailLog": fmt.Sprintf("Email sent to %s with subject: %s", emailInput.RecipientEmail, emailInput.Subject),
	}

	// Return success status
	return batchsqlc.StatusEnumSuccess, batch.JSONstr(`{"message": "Email sent successfully"}`), nil, blobRows, nil
}

type EmailInitializer struct{}

func (i *EmailInitializer) Init(app string) (batch.InitBlock, error) {
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
	queries := batchsqlc.New(pool)

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Create a new Batch instance
	batchClient := batch.Batch{
		Db:          pool,
		Queries:     queries,
		RedisClient: redisClient,
	}

	// Register the batch processor and initializer
	err := batch.RegisterProcessor("emailapp", "sendbulkemail", &EmailBatchProcessor{})
	if err != nil {
		log.Fatal("Failed to register batch processor:", err)
	}

	err = batch.RegisterInitializer("emailapp", &EmailInitializer{})
	if err != nil {
		log.Fatal("Failed to register initializer:", err)
	}

	// Prepare the batch input data
	batchInput := []batchsqlc.InsertIntoBatchRowsParams{
		{
			Line:  1,
			Input: []byte(`{"recipientEmail": "user1@example.com", "subject": "Batch Email 1", "body": "Hello, this is batch email 1."}`),
		},
		{
			Line:  2,
			Input: []byte(`{"recipientEmail": "user2@example.com", "subject": "Batch Email 2", "body": "Hello, this is batch email 2."}`),
		},
		// Add more batch input records as needed
	}

	// Submit the batch
	batchID, err := batchClient.Submit("emailapp", "sendbulkemail", batch.JSONstr("{}"), batchInput)
	if err != nil {
		log.Fatal("Failed to submit batch:", err)
	}

	fmt.Println("Batch submitted. Batch ID:", batchID)

	// Start the JobManager in a separate goroutine
	go batch.JobManager(pool, redisClient)

	// Poll for the batch completion status
	for {
		status, batchOutput, outputFiles, nsuccess, nfailed, naborted, err := batchClient.Done(batchID)
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
		fmt.Println("Batch output:", batchOutput)
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

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatal("Error connecting to the database:", err)
	}
	return pool
}
