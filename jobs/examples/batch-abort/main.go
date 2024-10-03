package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/examples"
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
		emptyJson, _ := jobs.NewJSONstr("")
		return batchsqlc.StatusEnumFailed, emptyJson, nil, nil, err
	}

	// Simulate sending the email
	fmt.Printf("Sending email to %s with subject: %s\n", emailInput.RecipientEmail, emailInput.Subject)
	time.Sleep(5 * time.Second) // Simulating email sending delay

	// Generate sample blobRows data
	blobRows = map[string]string{
		"emailLog": fmt.Sprintf("Email sent to %s with subject: %s", emailInput.RecipientEmail, emailInput.Subject),
	}

	// Return success status
	result, _ = jobs.NewJSONstr(`{"message": "Email sent successfully"}`)
	return batchsqlc.StatusEnumSuccess, result, nil, blobRows, nil
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

func (ib *EmailInitBlock) IsAlive() (bool, error) {
	// Implement the logic to check if the EmailInitBlock is alive
	return false, nil
}
func main() {
	// Initialize the database connection
	pool := getDb()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	store := examples.CreateMinioClient()
	lctx := logharbour.NewLoggerContext(logharbour.DefaultPriority)
	logger := logharbour.NewLogger(lctx, "JobManager", os.Stdout)

	// Initialize JobManager
	jm := jobs.NewJobManager(pool, redisClient, store, logger, nil)

	// Register the batch processor and initializer
	err := jm.RegisterProcessorBatch("emailapp", "sendbulkemail", &EmailBatchProcessor{})
	if err != nil {
		log.Fatal("Failed to register batch processor:", err)
	}

	err = jm.RegisterInitializer("emailapp", &EmailInitializer{})
	if err != nil {
		log.Fatal("Failed to register initializer:", err)
	}

	// Define the base email template with placeholders for dynamic content
	emailTemplate := examples.EmailTemplate{
		RecipientEmail: "user",
		Subject:        "Batch Email",
		Body:           "Hello, this is a batch email.",
	}

	// Specify the desired number of records to generate
	numRecords := 100 // Example: Generate 100 unique email records

	// Generate the batch input data using the template and number of records
	batchInput := examples.GenerateBatchInput(numRecords, emailTemplate)
	// Submit the batch
	emptyJson, _ := jobs.NewJSONstr("{}")
	batchID, err := jm.BatchSubmit("emailapp", "sendbulkemail", emptyJson, batchInput, false)
	if err != nil {
		log.Fatal("Failed to submit batch:", err)
	}

	fmt.Println("Batch submitted. Batch ID:", batchID)

	// Start the JobManager in a separate goroutine
	go jm.Run()

	// Wait for a short duration before aborting the batch
	time.Sleep(15 * time.Second)

	// Abort the batch
	status, nsuccess, nfailed, naborted, err := jm.BatchAbort(batchID)
	if err != nil {
		log.Fatal("Failed to abort batch:", err)
	}

	fmt.Println("Batch aborted. Status:", status)
	fmt.Println("Success count:", nsuccess)
	fmt.Println("Failed count:", nfailed)
	fmt.Println("Aborted count:", naborted)

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
