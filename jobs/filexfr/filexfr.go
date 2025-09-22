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
	"github.com/remiges-tech/logharbour/logharbour"
)

// FileChk is the type for file checking functions
type FileChk func(fileContents string, fileName string, batchctx jobs.JSONstr) (bool, jobs.JSONstr, []jobs.BatchInput_t, string, string, string)

// FileXfrConfig holds configuration for file transfer operations
type FileXfrConfig struct {
	// MaxObjectIDLength sets the maximum length for object IDs in the object store.
	// Object IDs are derived from sanitized filenames and truncated to this length.
	// Default: 500 characters. S3/MinIO limit is 1024 bytes.
	MaxObjectIDLength int

	// IncomingBucket is the bucket name for storing incoming files.
	// Default: "incoming"
	IncomingBucket    string

	// FailedBucket is the bucket name for storing files that failed processing.
	// Default: "failed"
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

	// objStore interfaces with the object storage system
	objStore objstore.ObjectStore

	// mu protects concurrent access to fileChkMap
	mu sync.RWMutex

	// queries provides database operations for batch-related tables
	queries batchsqlc.Querier

	// config holds file transfer configuration
	config FileXfrConfig

	// logger is the LogHarbour logger instance
	logger *logharbour.Logger
}

// NewFileXfrServer creates a new FileXfrServer with the given configuration
func NewFileXfrServer(jobManager *jobs.JobManager, objStore objstore.ObjectStore, queries batchsqlc.Querier, config FileXfrConfig, logger *logharbour.Logger) *FileXfrServer {
	if config.MaxObjectIDLength == 0 {
		config.MaxObjectIDLength = 500 // Default value if not specified
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
		logger:     logger,
	}
}

// RegisterFileChk registers a file checking function for a file type.
// Each file type can have only one function.
// Returns error if file type already registered.
func (fxs *FileXfrServer) RegisterFileChk(fileType string, fileChkFn FileChk) error {
	fxs.mu.Lock()
	defer fxs.mu.Unlock()

	if _, exists := fxs.fileChkMap[fileType]; exists {
		return fmt.Errorf("file check function already registered for file type: %s", fileType)
	}

	fxs.fileChkMap[fileType] = fileChkFn
	fxs.logger.Debug2().LogActivity("Registered file check function", map[string]any{
		"fileType": fileType,
	})
	return nil
}

// BulkfileinProcess handles the processing of incoming batch files.
// The 'file' parameter can be either file contents or an object ID,
// controlled by the 'isObjectID' boolean parameter.
func (fxs *FileXfrServer) BulkfileinProcess(file, filename, filetype string, batchctx jobs.JSONstr, isObjectID bool) (string, error) {
	var fileContents string
	var objectID string

	if isObjectID {
		objectID = file
		var err error
		fileContents, err = fxs.getObjectContents(objectID)
		if err != nil {
			fxs.logger.Debug2().LogActivity("Failed to read object contents", map[string]any{
				"objectID": objectID,
				"error":    err.Error(),
			})
			return "", fmt.Errorf("failed to read object contents: %v", err)
		}
	} else {
		fileContents = file
	}

	fileChkFn, exists := fxs.fileChkMap[filetype]
	if !exists {
		fxs.logger.Debug2().LogActivity("No file check function registered", map[string]any{
			"filetype": filetype,
		})
		return "", fmt.Errorf("no file check function registered for file type: %s", filetype)
	}

	isgood, batchctx, batchInput, app, op, _ := fileChkFn(fileContents, filename, batchctx)

	if !isgood {
		if objectID != "" {
			if err := fxs.moveObjectToFailedBucket(objectID); err != nil {
				fxs.logger.Debug2().LogActivity("Failed to move object to failed bucket", map[string]any{
					"objectID": objectID,
					"error":    err.Error(),
				})
				return "", fmt.Errorf("failed to move object to failed bucket: %v", err)
			}
		}
		fxs.logger.Debug2().LogActivity("File check failed", map[string]any{
			"filetype": filetype,
			"filename": filename,
		})
		return "", fmt.Errorf("file check failed for file type: %s", filetype)
	}

	batchID, err := fxs.jobManager.BatchSubmit(app, op, batchctx, batchInput, false)
	if err != nil {
		fxs.logger.Debug2().LogActivity("Failed to submit batch", map[string]any{
			"app":     app,
			"op":      op,
			"context": batchctx,
			"error":   err.Error(),
		})
		return "", fmt.Errorf("failed to submit batch: %v", err)
	}

	if objectID == "" {
		objectID, err = fxs.storeFileContents(fileContents, filename)
		if err != nil {
			fxs.logger.Debug2().LogActivity("Failed to store file contents", map[string]any{
				"filename": filename,
				"error":    err.Error(),
			})
			return "", fmt.Errorf("failed to store file contents: %v", err)
		}
	}

	if err := fxs.recordBatchFile(objectID, len(fileContents), batchID, isgood); err != nil {
		fxs.logger.Debug2().LogActivity("Failed to record batch file", map[string]any{
			"objectID": objectID,
			"batchID":  batchID,
			"error":    err.Error(),
		})
		return "", fmt.Errorf("failed to record batch file: %v", err)
	}

	fxs.logger.Debug2().LogActivity("Successfully processed file", map[string]any{
		"filetype": filetype,
		"filename": filename,
		"batchID":  batchID,
	})
	return batchID, nil
}

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

	objectID := fxs.generateObjectID(filename)
	reader := strings.NewReader(contents)

	err := fxs.objStore.Put(ctx, fxs.config.IncomingBucket, objectID, reader, int64(len(contents)), detectContentType(contents, filename))
	if err != nil {
		return "", fmt.Errorf("failed to store file contents: %w", err)
	}

	return objectID, nil
}

// generateObjectID creates an object ID for storing in the object store.
// The ID is derived from the sanitized filename and truncated to MaxObjectIDLength if needed.
//
// Note: Object IDs are not guaranteed to be unique - files with identical names
// (after sanitization) will have the same object ID and may cause conflicts.
// Users should ensure filenames are unique or include distinguishing elements
// like timestamps in their filenames.
func (fxs *FileXfrServer) generateObjectID(filename string) string {
	sanitizedFilename := sanitizeFilename(filename)

	if len(sanitizedFilename) > fxs.config.MaxObjectIDLength {
		sanitizedFilename = sanitizedFilename[:fxs.config.MaxObjectIDLength]
	}

	return sanitizedFilename
}

// sanitizeFilename removes or replaces characters that might be problematic in object storage
func sanitizeFilename(filename string) string {
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(filename)
}

// recordBatchFile writes a record in the batch-files table
func (fxs *FileXfrServer) recordBatchFile(objectID string, size int, batchID string, status bool) error {
	ctx := context.Background()

	// TODO: Calculate checksum from actual file contents instead of objectID
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
