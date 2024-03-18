package objstore

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

// ObjectStore is a generic interface for object store operations
type ObjectStore interface {
	Put(ctx context.Context, bucket, obj string, reader io.Reader, size int64, contentType string) error
	Get(ctx context.Context, bucket, obj string) (io.ReadCloser, error)
}

// MinioObjectStore is an implementation of ObjectStore using Minio
type MinioObjStore struct {
	client *minio.Client
}

// NewMinioObjectStore creates a new instance of MinioObjectStore with the provided Minio client
func NewMinioObjectStore(client *minio.Client) *MinioObjStore {
	return &MinioObjStore{client: client}
}

// Put uploads an object to Minio
func (s *MinioObjStore) Put(ctx context.Context, bucket, obj string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, bucket, obj, reader, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

// Get retrieves an object from Minio
func (s *MinioObjStore) Get(ctx context.Context, bucket, obj string) (io.ReadCloser, error) {
	return s.client.GetObject(ctx, bucket, obj, minio.GetObjectOptions{})
}
