// Package filexfr provides functionality for file transfer and processing in alya batch operations.
//
// This file (infiled.go) contains the implementation of the Infiled daemon, which monitors
// directories for incoming files and processes them using the FileXfrServer.
//
// Structs:
//   - FileTypeMapping: Represents a single file type mapping from the JSON configuration.
//     It maps a file path pattern to a specific file type.
//   - InfiledConfig: Configuration for the Infiled daemon, including watch directories,
//     file type mappings, sleep interval, and file age threshold.
//   - Infiled: The main struct representing the Infiled daemon, containing configuration,
//     FileXfrServer reference, and a map to track processed files.
//
// Functions:
//   - NewInfiled: Creates a new Infiled instance with the given configuration and FileXfrServer.
//   - Run: The main loop of the Infiled daemon, periodically polling directories for new files.
//   - processAllMappings: Processes all file type mappings defined in the configuration.
//   - processSingleMapping: Processes a single file type mapping.
//   - findFiles: Finds all files matching the given pattern in the watch directories.
//   - processFile: Processes a single file.
//   - isFileOldEnough: Checks if the file is old enough to be processed.
//   - storeFileInIncomingBucket: Stores the file in the "incoming" bucket of the object store.
//   - moveObjectToFailedBucket: Moves an object from the "incoming" bucket to the "failed" bucket.
//
// The Infiled daemon operates by periodically checking specified directories for new files,
// processing eligible files based on age and type, and using the FileXfrServer to handle
// the actual file transfer and batch submission process.

package filexfr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/remiges-tech/alya/jobs"
	"github.com/remiges-tech/logharbour/logharbour"
)

// FileTypeMapping represents a single file type mapping from the JSON configuration.
// It maps a file path pattern to a specific file type.
type FileTypeMapping struct {
	Path string `json:"path"` // The file path pattern to match
	Type string `json:"type"` // The corresponding file type
}

// InfiledConfig holds the configuration for the Infiled daemon.
// It includes settings for directories to watch, file type mappings, and timing parameters.
type InfiledConfig struct {
	WatchDirs     []string          // List of directories to monitor for incoming files
	FileTypeMap   []FileTypeMapping // Slice of file type mappings
	SleepInterval time.Duration     // Duration to wait between processing cycles
	FileAgeSecs   int               // Minimum age (in seconds) of files to be processed
}

// Infiled represents the Infiled daemon.
// It contains the configuration, a reference to the FileXfrServer, and a map to track processed files.
type Infiled struct {
	config         InfiledConfig
	fxs            *FileXfrServer
	processedFiles map[string]time.Time
	mu             sync.Mutex         // Mutex to protect concurrent access to processedFiles
	logger         *logharbour.Logger // Logger instance for logging
}

// NewInfiled creates and returns a new Infiled instance.
// It initializes the Infiled with the provided configuration and FileXfrServer.
func NewInfiled(config InfiledConfig, fxs *FileXfrServer, logger *logharbour.Logger) (*Infiled, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	return &Infiled{
		config:         config,
		fxs:            fxs,
		processedFiles: make(map[string]time.Time),
		logger:         logger,
	}, nil
}

// Run starts the Infiled daemon.
// It runs in an infinite loop, processing file mappings at regular intervals.
func (i *Infiled) Run() error {
	if i == nil {
		return fmt.Errorf("Infiled instance is nil")
	}
	if i.logger == nil {
		return fmt.Errorf("logger is not initialized")
	}
	i.logger.Info().LogActivity("Starting Infiled daemon", nil)
	for {
		if err := i.processAllMappings(); err != nil {
			i.logger.Error(err).LogActivity("Error processing mappings", nil)
		}
		time.Sleep(i.config.SleepInterval)
	}
}

// processAllMappings processes all file type mappings defined in the configuration.
// It iterates through each mapping and processes files that match the mapping's pattern.
func (i *Infiled) processAllMappings() error {
	if i == nil || i.logger == nil {
		return fmt.Errorf("Infiled instance or logger is nil")
	}
	for _, mapping := range i.config.FileTypeMap {
		if err := i.processSingleMapping(mapping); err != nil {
			i.logger.Error(err).LogActivity("Error processing mapping", map[string]any{"mapping": mapping})
		}
	}
	return nil
}

// processSingleMapping processes a single file type mapping.
// It finds all files matching the mapping's pattern and processes each file.
func (i *Infiled) processSingleMapping(mapping FileTypeMapping) error {
	files, err := i.findFiles(mapping.Path)
	i.logger.Info().LogActivity("Found files matching pattern", map[string]any{"count": len(files), "pattern": mapping.Path})
	if err != nil {
		return fmt.Errorf("error finding files for pattern %s: %w", mapping.Path, err)
	}

	for _, file := range files {
		i.logger.Info().LogActivity("Processing file", map[string]any{"file": file})
		if err := i.processFile(file, mapping.Type); err != nil {
			i.logger.Error(err).LogActivity("Error processing file", map[string]any{"file": file})
		}
	}

	return nil
}

// findFiles finds all files matching the given pattern in the watch directories.
// It uses the doublestar library to support advanced glob patterns.
//
// We use the doublestar library instead of the standard filepath.Glob for several reasons:
// 1. It supports more advanced globbing patterns, including '**' for recursive matching.
// 2. It can handle patterns like '/*/incoming/txnbatch/TXN*.xlsx' which filepath.Glob cannot.
// 3. It provides a closer match to Unix-style glob functionality.
// 4. It allows us to match files across multiple directory levels without complex custom logic.
func (i *Infiled) findFiles(pattern string) ([]string, error) {
	i.logger.Info().LogActivity("Finding files matching pattern", map[string]any{"pattern": pattern, "directories": i.config.WatchDirs})
	var files []string
	for _, dir := range i.config.WatchDirs {
		// Use os.DirFS rooted at dir
		fs := os.DirFS(dir)
		i.logger.Info().LogActivity("Using os.DirFS with root", map[string]any{"root": dir})

		// When using fs.Glob, the pattern should be relative to dir
		matches, err := doublestar.Glob(fs, pattern)
		if err != nil {
			return nil, fmt.Errorf("error globbing pattern %s in directory %s: %w", pattern, dir, err)
		}

		i.logger.Info().LogActivity("Found matches", map[string]any{"matches": matches})
		// Prepend dir to each match to get the full path
		for _, match := range matches {
			fullPath := filepath.Join(dir, match)
			files = append(files, fullPath)
		}
	}

	i.logger.Info().LogActivity("Total files found", map[string]any{"count": len(files)})
	return files, nil
}

// processFile processes a single file.
// It checks the file's age, stores it in the object store, calls BulkfileinProcess,
// and handles any errors that occur during processing.
func (i *Infiled) processFile(filePath, fileType string) error {
	// Check if the file is old enough to be processed
	if !i.isFileOldEnough(filePath) {
		return nil // File is too new, skip it
	}

	// Store the file in the "incoming" bucket of the object store
	objectID, err := i.storeFileInIncomingBucket(filePath)
	if err != nil {
		return fmt.Errorf("error storing file %s: %w", filePath, err)
	}

	// Process the file using BulkfileinProcess
	batchctx, _ := jobs.NewJSONstr("{}")
	batchID, err := i.fxs.BulkfileinProcess(objectID, filepath.Base(filePath), fileType, batchctx, true)
	if err != nil {
		// If processing fails, move the object to the "failed" bucket
		if moveErr := i.fxs.moveObjectToFailedBucket(objectID); moveErr != nil {
			i.logger.Error(moveErr).LogActivity("Error moving object to failed bucket", map[string]any{"objectID": objectID, "error": moveErr.Error()})
		}
		return fmt.Errorf("error processing file %s: %w", filePath, err)
	}

	i.logger.Info().LogActivity("Successfully processed file", map[string]any{
		"filePath": filePath,
		"fileType": fileType,
		"objectID": objectID,
		"batchID":  batchID,
	})

	// Delete the file from the file system after successful processing
	if err := os.Remove(filePath); err != nil {
		i.logger.Error(err).LogActivity("Error deleting file", map[string]any{"file": filePath})
	}

	return nil
}

// isFileOldEnough checks if the file is old enough to be processed.
// It compares the file's modification time with the current time and the configured age threshold.
func (i *Infiled) isFileOldEnough(filePath string) bool {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		i.logger.Error(err).LogActivity("Error getting file info", map[string]any{"file": filePath})
		return false
	}

	return time.Since(fileInfo.ModTime()) >= time.Duration(i.config.FileAgeSecs)*time.Second
}

// storeFileInIncomingBucket stores the file in the incoming bucket
func (i *Infiled) storeFileInIncomingBucket(filePath string) (string, error) {
	i.logger.Info().LogActivity("Storing file in incoming bucket", map[string]any{"file": filePath, "bucket": i.fxs.config.IncomingBucket})
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file %s: %w", filePath, err)
	}
	defer file.Close()

	// Use the file's base name as the object ID
	objectID := filepath.Base(filePath)

	// Store the file in the incoming bucket
	err = i.fxs.objStore.Put(context.TODO(), i.fxs.config.IncomingBucket, objectID, file, -1, "application/octet-stream")
	if err != nil {
		return "", fmt.Errorf("error storing object %s: %w", objectID, err)
	}

	return objectID, nil
}

// moveObjectToFailedBucket moves an object from the incoming bucket to the failed bucket
func (i *Infiled) moveObjectToFailedBucket(objectID string) error {
	ctx := context.Background()

	// Get the object from the incoming bucket
	srcObject, err := i.fxs.objStore.Get(ctx, i.fxs.config.IncomingBucket, objectID)
	if err != nil {
		return fmt.Errorf("error getting object %s from incoming bucket: %w", objectID, err)
	}
	defer srcObject.Close()

	// Use the same object ID when moving to the failed bucket
	failedObjectID := objectID

	// Put the object in the failed bucket
	err = i.fxs.objStore.Put(ctx, i.fxs.config.FailedBucket, failedObjectID, srcObject, -1, "application/octet-stream")
	if err != nil {
		return fmt.Errorf("error storing object %s in failed bucket: %w", failedObjectID, err)
	}

	// Remove the original object from the incoming bucket
	err = i.fxs.objStore.Delete(ctx, i.fxs.config.IncomingBucket, objectID)
	if err != nil {
		i.logger.Error(err).LogActivity("Error removing object from incoming bucket", map[string]any{"objectID": objectID})
	}

	return nil
}
