package filexfr

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/alya/jobs/objstore"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
)

// FileChk is the type for file checking functions
type FileChk func(fileContents string, fileName string) (bool, jobs.JSONstr, []jobs.BatchInput_t, string, string, string)

// FileXfrConfig holds configuration for file transfer operations
type FileXfrConfig struct {
	MaxObjectIDLength int
	IncomingBucket    string
	FailedBucket      string
}

// FileXfrServer handles file transfer operations
type FileXfrServer struct {
	// fileChkMap stores file checking functions for each file type
	// Key: file type (e.g., "banktransactions", "customerdata")
	// Value: function to validate and process files of that type
	fileChkMap map[string]FileChk

	// jobManager manages alya batch jobs
	// FileXfrServer will submit batch jobs using the jobManager
	jobManager *jobs.JobManager

	// objStore interfaces with the object storage system -- in this case, Minio
	// It handles storing and retrieving file contents
	objStore objstore.ObjectStore

	// mu is a mutex for thread-safe access to shared resources
	// It prevents concurrent modifications to the fileChkMap
	mu sync.RWMutex

	// queries provides database operations for batch-related tables
	queries batchsqlc.Querier

	// config holds configuration settings for file transfer operations
	// It includes settings like max object ID length and bucket name
	config FileXfrConfig
}

// NewFileXfrServer creates a new FileXfrServer with the given configuration
func NewFileXfrServer(jobManager *jobs.JobManager, objStore objstore.ObjectStore, queries batchsqlc.Querier, config FileXfrConfig) *FileXfrServer {
	if config.MaxObjectIDLength == 0 {
		config.MaxObjectIDLength = 200 // Default value if not specified
	}
	if config.IncomingBucket == "" {
		config.IncomingBucket = "incoming" // Default incoming bucket name
	}
	if config.FailedBucket == "" {
		config.FailedBucket = "failed" // Default failed bucket name
	}
	return &FileXfrServer{
		fileChkMap: make(map[string]FileChk),
		jobManager: jobManager,
		objStore:   objStore,
		queries:    queries,
		config:     config,
	}
}

// RegisterFileChk allows applications to register a file checking function for a specific file type.
// Each file type can only have one registered file checking function.
// Attempting to register a second function for the same file type will result in an error.
func (fxs *FileXfrServer) RegisterFileChk(fileType string, fileChkFn FileChk) error {
	fxs.mu.Lock()
	defer fxs.mu.Unlock()

	if _, exists := fxs.fileChkMap[fileType]; exists {
		return fmt.Errorf("file check function already registered for file type: %s", fileType)
	}

	fxs.fileChkMap[fileType] = fileChkFn
	return nil
}

// BulkfileinProcess handles the processing of incoming batch files
func (fxs *FileXfrServer) BulkfileinProcess(file, filename, filetype string) error {
	var fileContents string
	var objectID string

	// Check if the input is an object ID or file contents
	if len(file) < fxs.config.MaxObjectIDLength {
		// Assume it's an object ID
		objectID = file
		var err error
		fileContents, err = fxs.getObjectContents(objectID)
		if err != nil {
			return fmt.Errorf("failed to read object contents: %v", err)
		}
	} else {
		fileContents = file
	}

	// Get the registered file check function for the given file type
	fileChkFn, exists := fxs.fileChkMap[filetype]
	if !exists {
		return fmt.Errorf("no file check function registered for file type: %s", filetype)
	}

	// Call the file check function
	isgood, context, batchInput, app, op, _ := fileChkFn(fileContents, filename)

	if !isgood {
		// Move the object to the "failed" bucket if it exists
		if objectID != "" {
			if err := fxs.moveObjectToFailedBucket(objectID); err != nil {
				return fmt.Errorf("failed to move object to failed bucket: %v", err)
			}
		}
		return fmt.Errorf("file check failed for file type: %s", filetype)
	}

	// Submit the batch using JobManager
	batchID, err := fxs.jobManager.BatchSubmit(app, op, context, batchInput, false)
	if err != nil {
		return fmt.Errorf("failed to submit batch: %v", err)
	}

	// If file contents were given in the request, not an object ID, store it in the object store
	if objectID == "" {
		objectID, err = fxs.storeFileContents(fileContents, filename)
		if err != nil {
			return fmt.Errorf("failed to store file contents: %v", err)
		}
	}

	// Write a record in the batch-files table
	if err := fxs.recordBatchFile(objectID, len(fileContents), batchID, isgood); err != nil {
		return fmt.Errorf("failed to record batch file: %v", err)
	}

	return nil
}

// Helper functions

// getObjectContents retrieves the contents of an object from the object store
func (fxs *FileXfrServer) getObjectContents(objectID string) (string, error) {
	ctx := context.Background()

	reader, err := fxs.objStore.Get(ctx, fxs.config.IncomingBucket, objectID)
	if err != nil {
		return "", fmt.Errorf("failed to get object from store: %w", err)
	}
	defer reader.Close()

	contents, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read object contents: %w", err)
	}

	return string(contents), nil
}

// moveObjectToFailedBucket moves an object from the incoming bucket to the failed bucket
func (fxs *FileXfrServer) moveObjectToFailedBucket(objectID string) error {
	ctx := context.Background()

	// Get the object from the incoming bucket
	reader, err := fxs.objStore.Get(ctx, fxs.config.IncomingBucket, objectID)
	if err != nil {
		return fmt.Errorf("failed to get object %s from incoming bucket: %w", objectID, err)
	}
	defer reader.Close()

	// Put the object in the failed bucket using the same object ID
	err = fxs.objStore.Put(ctx, fxs.config.FailedBucket, objectID, reader, -1, "application/octet-stream")
	if err != nil {
		return fmt.Errorf("failed to put object %s in failed bucket: %w", objectID, err)
	}

	// Delete the original object from the incoming bucket
	err = fxs.objStore.Delete(ctx, fxs.config.IncomingBucket, objectID)
	if err != nil {
		return fmt.Errorf("failed to delete object %s from incoming bucket: %w", objectID, err)
	}

	return nil
}

// storeFileContents stores the file contents in the object store and returns the object ID
func (fxs *FileXfrServer) storeFileContents(contents, filename string) (string, error) {
	ctx := context.Background()

	// Use the sanitized filename as the object ID
	objectID := fxs.generateObjectID(filename) // Ensure this generates just the filename

	// Create a reader from the file contents
	reader := strings.NewReader(contents)

	// Store the object in the incoming bucket
	err := fxs.objStore.Put(ctx, fxs.config.IncomingBucket, objectID, reader, int64(len(contents)), detectContentType(contents, filename))
	if err != nil {
		return "", fmt.Errorf("failed to store file contents: %w", err)
	}

	return objectID, nil
}

// generateObjectID creates a unique object ID for storing in the object store
func (fxs *FileXfrServer) generateObjectID(filename string) string {
	// Sanitize the filename to remove problematic characters
	sanitizedFilename := sanitizeFilename(filename)

	// Truncate if necessary based on MaxObjectIDLength
	if len(sanitizedFilename) > fxs.config.MaxObjectIDLength {
		sanitizedFilename = sanitizedFilename[:fxs.config.MaxObjectIDLength]
	}

	return sanitizedFilename
}

// sanitizeFilename removes or replaces characters that might be problematic in object storage
func sanitizeFilename(filename string) string {
	// Replace spaces and other potentially problematic characters
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	sanitized := replacer.Replace(filename)

	// Truncate if the filename is too long (adjust the max length as needed)
	maxLength := 100
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
	}

	return sanitized
}

// recordBatchFile writes a record in the batch-files table
func (fxs *FileXfrServer) recordBatchFile(objectID string, size int, batchID string, status bool) error {
	ctx := context.Background()

	// Generate a checksum for the file (you might want to implement a proper checksum function)
	// Calculate MD5 checksum
	h := md5.New()
	_, err := io.WriteString(h, objectID)
	if err != nil {
		return fmt.Errorf("failed to calculate MD5 checksum: %v", err)
	}
	checksum := fmt.Sprintf("%x", h.Sum(nil))

	// Convert batchID string to UUID
	batchUUID, err := uuid.Parse(batchID)
	if err != nil {
		return fmt.Errorf("invalid batch ID: %v", err)
	}

	// Insert the record into the batch-files table
	err = fxs.queries.InsertBatchFile(ctx, batchsqlc.InsertBatchFileParams{
		ObjectID:   objectID,
		Size:       int64(size),
		Checksum:   checksum,
		ReceivedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Status:     status,
		BatchID:    batchUUID,
	})

	if err != nil {
		return fmt.Errorf("failed to insert batch file record: %v", err)
	}

	return nil
}

// detectContentType determines the content type of the file using mimetype package
func detectContentType(contents, filename string) string {
	// Detect MIME type from content
	mtype := mimetype.Detect([]byte(contents))
	detectedType := mtype.String()

	// If the detected type is too generic, try to refine it using the file extension
	if detectedType == "application/octet-stream" || detectedType == "text/plain" {
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".csv":
			return "text/csv"
		case ".tsv":
			return "text/tab-separated-values"
		case ".json":
			return "application/json"
		case ".xml":
			return "application/xml"
		case ".yaml", ".yml":
			return "application/x-yaml"
		}
	}

	return detectedType
}
