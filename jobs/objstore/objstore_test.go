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

	ctx := context.Background()
	bucketName := "testbucket"
	if _, err := minioClient.BucketExists(ctx, bucketName); err != nil {
		t.Skipf("minio not available on localhost:9000: %v", err)
	}

	// Create a new MinioObjectStore instance with the provided Minio client
	store := objstore.NewMinioObjectStore(minioClient)

	// Create the test bucket
	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		// Check if the bucket already exists
		exists, err := minioClient.BucketExists(ctx, bucketName)
		if err != nil || !exists {
			t.Fatalf("Error creating test bucket: %v", err)
		}
	}

	// Create a test object
	objectName := "test-object"
	objectContent := []byte("Hello, World!")

	// Put the test object
	err = store.Put(ctx, bucketName, objectName, bytes.NewReader(objectContent), int64(len(objectContent)), "text/plain")
	if err != nil {
		t.Fatalf("Error putting object: %v", err)
	}

	// Get the test object
	reader, err := store.Get(ctx, bucketName, objectName)
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
