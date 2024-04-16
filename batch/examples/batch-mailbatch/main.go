package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/remiges-tech/alya/batch"
	"github.com/remiges-tech/alya/batch/examples"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

type EmailBatchProcessor struct{}

func (p *EmailBatchProcessor) DoBatchJob(initBlock batch.InitBlock, context batch.JSONstr, line int, input batch.JSONstr) (status batchsqlc.StatusEnum, result batch.JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error) {
	// Parse the input JSON
	var emailInput examples.EmailInput
	err = json.Unmarshal([]byte(input), &emailInput)
	if err != nil {
		return batchsqlc.StatusEnumFailed, "", nil, nil, err
	}

	// Simulate sending the email
	fmt.Printf("Sending email to %s with subject %s\n", emailInput.RecipientEmail, emailInput.Subject)
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

	// Initialize JobManager
	jm := batch.NewJobManager(pool, redisClient, minioClient)

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
	batchID, err := jm.BatchSubmit("emailapp", "sendbulkemail", batch.JSONstr("{}"), batchInput, false)
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
