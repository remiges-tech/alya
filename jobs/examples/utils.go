package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/remiges-tech/alya/jobs"
)

// CreateMinioClient initializes and returns a Minio client.
func CreateMinioClient() *minio.Client {
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
		exists, errBucketExists := minioClient.BucketExists(context.Background(), bucketName)
		if errBucketExists != nil || !exists {
			log.Fatalf("Error creating test bucket: %v", err)
		}
	}
	return minioClient
}

// InitializeDatabase connects to the database and runs migrations.
func InitializeDatabase(connString string) (*pgx.Conn, error) {
	// Connect to the database
	conn, err := pgx.Connect(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	// Run database migrations
	err = jobs.MigrateDatabase(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %v", err)
	}

	return conn, nil
}

type EmailInput struct {
	RecipientEmail string `json:"recipientEmail"`
	Subject        string `json:"subject"`
	Body           string `json:"body"`
}

// EmailTemplate defines the structure for the base email template.
type EmailTemplate struct {
	RecipientEmail string
	Subject        string
	Body           string
}

// GenerateBatchInput creates a slice of BatchInput_t for batch processing.
// numRecords specifies the number of records to generate.
// emailTemplate is the base template for generating the email records.
func GenerateBatchInput(numRecords int, emailTemplate EmailTemplate) []jobs.BatchInput_t {
	var batchInput []jobs.BatchInput_t

	for i := 1; i <= numRecords; i++ {
		emailInput := EmailInput{
			RecipientEmail: fmt.Sprintf("%s%d@example.com", emailTemplate.RecipientEmail, i),
			Subject:        fmt.Sprintf("%s %d", emailTemplate.Subject, i),
			Body:           emailTemplate.Body,
		}

		emailInputBytes, err := json.Marshal(emailInput)
		if err != nil {
			// Error marshalling email input; skip this record
			fmt.Printf("Error marshalling email input: %v\n", err)
			continue
		}

		input, err := jobs.NewJSONstr(string(emailInputBytes))
		if err != nil {
			// Error creating JSON string; skip this record
			fmt.Printf("Error creating JSON string: %v\n", err)
			continue
		}
		batchInput = append(batchInput, jobs.BatchInput_t{
			Line:  i,
			Input: input,
		})
	}

	return batchInput
}
