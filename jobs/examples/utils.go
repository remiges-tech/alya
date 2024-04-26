package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
)

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
		exists, err := minioClient.BucketExists(context.Background(), bucketName)
		if err != nil || !exists {
			log.Fatalf("Error creating test bucket: %v", err)
		}
	}

	return minioClient
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

// GenerateBatchInput generates a slice of InsertIntoBatchRowsParams for batch processing.
// numRecords specifies the number of records to generate.
// emailTemplate is the base template for generating the email records.
func GenerateBatchInput(numRecords int, emailTemplate EmailTemplate) []batchsqlc.InsertIntoBatchRowsParams {
	var batchInput []batchsqlc.InsertIntoBatchRowsParams

	for i := 1; i <= numRecords; i++ {
		emailInput := EmailInput{
			RecipientEmail: fmt.Sprintf("%s%d@example.com", emailTemplate.RecipientEmail, i),
			Subject:        fmt.Sprintf("%s %d", emailTemplate.Subject, i),
			Body:           emailTemplate.Body,
		}

		emailInputBytes, err := json.Marshal(emailInput)
		if err != nil {
			// Handle error (for simplicity, we'll just log it here, but you might want to handle it differently)
			fmt.Printf("Error marshalling email input: %v\n", err)
			continue
		}

		batchInput = append(batchInput, batchsqlc.InsertIntoBatchRowsParams{
			Line:  int32(i),
			Input: emailInputBytes,
		})
	}

	return batchInput
}
