package filexfr

import (
	"bytes"
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
}

// FileXfrServer handles file transfer operations
type FileXfrServer struct {
	fileChkMap map[string]FileChk
	jobManager *jobs.JobManager
	objStore   objstore.ObjectStore
	mu         sync.RWMutex
	queries    batchsqlc.Querier
	config     FileXfrConfig
}

// NewFileXfrServer creates a new FileXfrServer with the given configuration
func NewFileXfrServer(jobManager *jobs.JobManager, objStore objstore.ObjectStore, queries batchsqlc.Querier, config FileXfrConfig) *FileXfrServer {
	if config.MaxObjectIDLength == 0 {
		config.MaxObjectIDLength = 200 // Default value if not specified
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
	// Create a context for the operation
	ctx := context.Background()

	// Get the object from the object store
	reader, err := fxs.objStore.Get(ctx, "incoming", objectID)
	if err != nil {
		return "", fmt.Errorf("failed to get object from store: %v", err)
	}
	defer reader.Close()

	// Read the contents of the object
	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, reader)
	if err != nil {
		return "", fmt.Errorf("failed to read object contents: %v", err)
	}

	return buffer.String(), nil
}

// moveObjectToFailedBucket moves an object from the incoming bucket to the failed bucket
func (fxs *FileXfrServer) moveObjectToFailedBucket(objectID string) error {
	ctx := context.Background()

	// Get the object from the incoming bucket
	reader, err := fxs.objStore.Get(ctx, "incoming", objectID)
	if err != nil {
		return fmt.Errorf("failed to get object from incoming bucket: %v", err)
	}
	defer reader.Close()

	// Generate a new object ID for the failed bucket
	failedObjectID := generateFailedObjectID(objectID)

	// Put the object in the failed bucket
	err = fxs.objStore.Put(ctx, "failed", failedObjectID, reader, -1, "application/octet-stream")
	if err != nil {
		return fmt.Errorf("failed to put object in failed bucket: %v", err)
	}

	// TODO: Implement deletion of the original object from the incoming bucket
	// This step depends on whether your object store supports atomic move operations
	// or if you need to implement it as a copy-then-delete operation

	return nil
}

// generateFailedObjectID creates a new object ID for the failed bucket
func generateFailedObjectID(originalID string) string {
	timestamp := time.Now().Format("20060102-150405")
	uniqueID := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%s_%s", filepath.Base(originalID), timestamp, uniqueID)
}

// storeFileContents stores the file contents in the object store and returns the object ID
func (fxs *FileXfrServer) storeFileContents(contents, filename string) (string, error) {
	ctx := context.Background()

	// Generate a unique object ID
	objectID := fxs.generateObjectID(filename)

	// Create a reader from the file contents
	reader := strings.NewReader(contents)

	// Determine the content type
	contentType := detectContentType(contents, filename)

	// Store the object in the "incoming" bucket
	err := fxs.objStore.Put(ctx, "incoming", objectID, reader, int64(len(contents)), contentType)
	if err != nil {
		return "", fmt.Errorf("failed to store file contents: %v", err)
	}

	return objectID, nil
}

// generateObjectID creates a unique object ID for storing in the object store
func (fxs *FileXfrServer) generateObjectID(filename string) string {
	sanitized := sanitizeFilename(filename)
	timestamp := time.Now().Format("20060102-150405")
	uniqueID := uuid.New().String()

	// Calculate the maximum length for the sanitized filename
	maxSanitizedLength := fxs.config.MaxObjectIDLength - len(timestamp) - len(uniqueID) - 2 // 2 for the underscores
	if maxSanitizedLength < 0 {
		maxSanitizedLength = 0
	}
	if len(sanitized) > maxSanitizedLength {
		sanitized = sanitized[:maxSanitizedLength]
	}

	objectID := fmt.Sprintf("%s_%s_%s", sanitized, timestamp, uniqueID)
	if len(objectID) > fxs.config.MaxObjectIDLength {
		// If it's still too long, truncate the uniqueID (least important for human readability)
		excessLength := len(objectID) - fxs.config.MaxObjectIDLength
		uniqueID = uniqueID[:len(uniqueID)-excessLength]
		objectID = fmt.Sprintf("%s_%s_%s", sanitized, timestamp, uniqueID)
	}

	return objectID
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
