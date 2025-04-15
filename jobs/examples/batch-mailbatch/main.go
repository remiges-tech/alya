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
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/examples"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
)

type EmailBatchProcessor struct {
}

func (p *EmailBatchProcessor) DoBatchJob(initBlock jobs.InitBlock, context jobs.JSONstr, line int, input jobs.JSONstr) (status batchsqlc.StatusEnum, result jobs.JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error) {
	// Parse the input JSON
	var emailInput examples.EmailInput
	err = json.Unmarshal([]byte(input.String()), &emailInput)
	if err != nil {
		emptyJson, _ := jobs.NewJSONstr("{}")
		return batchsqlc.StatusEnumFailed, emptyJson, nil, nil, err
	}

	// Simulate sending the email
	fmt.Printf("Sending email to %s with subject %s\n", emailInput.RecipientEmail, emailInput.Subject)
	time.Sleep(time.Second) // Simulating email sending delay

	// Generate sample blobRows data
	blobRows = map[string]string{
		"emailLog": fmt.Sprintf("Email sent to %s with subject: %s", emailInput.RecipientEmail, emailInput.Subject),
	}

	// Return success status
	result, _ = jobs.NewJSONstr(`{"message": "Email sent successfully"}`)
	return batchsqlc.StatusEnumSuccess, result, nil, blobRows, nil
}

// MarkDone is called when a batch completes. It can be used to:
// - Send batch completion notifications
// - Generate summary reports
// - Trigger downstream processes
// - Update application state
func (p *EmailBatchProcessor) MarkDone(initBlock jobs.InitBlock, context jobs.JSONstr, details jobs.BatchDetails_t) error {
	// Log batch completion details
	log.Printf("Email batch %s completed with status: %s", details.ID, details.Status)
	log.Printf("Successfully sent: %d", details.NSuccess)
	log.Printf("Failed to send: %d", details.NFailed)
	log.Printf("Aborted: %d", details.NAborted)

	// Generate and send a summary email to the administrator
	summaryEmail := examples.EmailInput{
		RecipientEmail: "admin@example.com",
		Subject:        fmt.Sprintf("Batch Email Summary - Batch ID: %s", details.ID),
		Body: fmt.Sprintf(`
Email Batch Summary:
------------------
Batch ID: %s
Status: %s
Emails Sent Successfully: %d
Failed Emails: %d
Aborted Emails: %d

Output Files:
`, details.ID, details.Status, details.NSuccess, details.NFailed, details.NAborted),
	}

	// Add output files information to the summary
	for filename, objectID := range details.OutputFiles {
		summaryEmail.Body += fmt.Sprintf("- %s: %s\n", filename, objectID)
	}

	// Simulate sending the summary email
	fmt.Printf("\nSending summary email:\n%s\n", summaryEmail.Body)

	// Example: Update a hypothetical email campaign tracking system
	if details.Status == batchsqlc.StatusEnumSuccess {
		fmt.Printf("Updating campaign status: All emails sent for batch %s\n", details.ID)
	} else {
		fmt.Printf("Updating campaign status: Batch %s completed with failures\n", details.ID)
	}

	// Example: If there were failures, create a retry batch for failed emails
	if details.NFailed > 0 {
		fmt.Printf("Creating retry batch for %d failed emails\n", details.NFailed)
		// In a real implementation, we would:
		// 1. Query the batch rows for failed entries
		// 2. Create a new batch with just the failed emails
		// 3. Submit the retry batch
	}

	// Example: Process any generated output files
	if len(details.OutputFiles) > 0 {
		fmt.Println("\nProcessing output files:")
		for logicalName, objectID := range details.OutputFiles {
			fmt.Printf("- Processing %s (Object ID: %s)\n", logicalName, objectID)
			// In a real implementation, we might:
			// - Download and parse the email logs
			// - Upload logs to a long-term storage system
			// - Generate detailed reports
			// - Update analytics systems
		}
	}

	// Example: Clean up any temporary resources
	fmt.Println("\nCleaning up temporary resources for batch", details.ID)

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

func (ib *EmailInitBlock) IsAlive() (bool, error) {
	// Check if the connection to the email server is alive
	return false, nil
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

	// Submit the batch with the generated input data
	emptyJson, _ := jobs.NewJSONstr("{}")
	batchID, err := jm.BatchSubmit("emailapp", "sendbulkemail", emptyJson, batchInput, false)
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

	// The MarkDone function will be automatically called by the job manager
	// when the batch completes. We can see its effects in the logs.

	// Add a sleep at the end of main to see the MarkDone output
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

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatal("Error connecting to the database:", err)
	}
	return pool
}
