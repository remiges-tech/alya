package objstore

import (
	"context"
	"io"
)

// ObjectStoreMock is a mock implementation of the ObjectStore interface.
type ObjectStoreMock struct {
	PutFunc    func(ctx context.Context, bucket, obj string, reader io.Reader, size int64, contentType string) error
	GetFunc    func(ctx context.Context, bucket, obj string) (io.ReadCloser, error)
	DeleteFunc func(ctx context.Context, bucket, obj string) error
}

// Put is a mock implementation of the Put method.
func (m *ObjectStoreMock) Put(ctx context.Context, bucket, obj string, reader io.Reader, size int64, contentType string) error {
	return m.PutFunc(ctx, bucket, obj, reader, size, contentType)
}

// Get is a mock implementation of the Get method.
func (m *ObjectStoreMock) Get(ctx context.Context, bucket, obj string) (io.ReadCloser, error) {
	return m.GetFunc(ctx, bucket, obj)
}

// Delete is a mock implementation of the Delete method.
func (m *ObjectStoreMock) Delete(ctx context.Context, bucket, obj string) error {
	return m.DeleteFunc(ctx, bucket, obj)
}

// GenerateObjectStoreMock generates a new mock instance of the ObjectStore interface.
func GenerateObjectStoreMock() *ObjectStoreMock {
	return &ObjectStoreMock{
		PutFunc: func(ctx context.Context, bucket, obj string, reader io.Reader, size int64, contentType string) error {
			return nil
		},
		GetFunc: func(ctx context.Context, bucket, obj string) (io.ReadCloser, error) {
			return nil, nil
		},
	}
}
