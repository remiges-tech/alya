package objstore_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/remiges-tech/alya/jobs/objstore"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestPutGetObject(t *testing.T) {
	// Create a new Minio client instance with the default credentials
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("Error creating Minio client: %v", err)
	}

	// Create a new MinioObjectStore instance with the provided Minio client
	store := objstore.NewMinioObjectStore(minioClient)

	// Create the test bucket
	bucketName := "testbucket"
	err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
	if err != nil {
		// Check if the bucket already exists
		exists, err := minioClient.BucketExists(context.Background(), bucketName)
		if err != nil || !exists {
			t.Fatalf("Error creating test bucket: %v", err)
		}
	}

	// Create a test object
	objectName := "test-object"
	objectContent := []byte("Hello, World!")

	// Put the test object
	err = store.Put(context.Background(), bucketName, objectName, bytes.NewReader(objectContent), int64(len(objectContent)), "text/plain")
	if err != nil {
		t.Fatalf("Error putting object: %v", err)
	}

	// Get the test object
	reader, err := store.Get(context.Background(), bucketName, objectName)
	if err != nil {
		t.Fatalf("Error getting object: %v", err)
	}
	defer reader.Close()

	// Read the object content
	retrievedContent := new(bytes.Buffer)
	_, err = retrievedContent.ReadFrom(reader)
	if err != nil {
		t.Fatalf("Error reading object: %v", err)
	}

	// Compare the retrieved content with the original content
	if !bytes.Equal(retrievedContent.Bytes(), objectContent) {
		t.Fatalf("Retrieved object content does not match the original content")
	}
}
