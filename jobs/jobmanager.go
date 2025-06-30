package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/remiges-tech/alya/jobs/objstore"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
	"github.com/remiges-tech/logharbour/logharbour"
)

// Error types for common processing failures
var (
	ErrProcessorNotFound    = errors.New("processor not found for this app and operation")
	ErrInitializerNotFound  = errors.New("initializer not found for this app")
	ErrInitializerFailed    = errors.New("initializer failed for this app")
	ErrInvalidProcessorType = errors.New("invalid processor type for this app and operation")
)

// ConfigurationError represents an error related to system configuration or setup
// such as missing processors, initializers, or type mismatches. These typically
// affect all jobs of a certain type and require admin intervention to resolve.
type ConfigurationError struct {
	BaseErr error  // The underlying error type
	App     string // The application name
	Op      string // The operation name, if applicable
	Details string // Additional error details
}

// Make sure ConfigurationError implements the error interface
func (e ConfigurationError) Error() string {
	if e.Op != "" {
		return fmt.Sprintf("%v for app %s and op %s: %s",
			e.BaseErr, e.App, e.Op, e.Details)
	}
	return fmt.Sprintf("%v for app %s: %s",
		e.BaseErr, e.App, e.Details)
}

// Allow unwrapping to get the base error
func (e ConfigurationError) Unwrap() error {
	return e.BaseErr
}

// ProcessorNotFoundError wraps ErrProcessorNotFound with app, op, and processor type details.
type ProcessorNotFoundError struct {
	App           string
	Op            string
	ProcessorType string // "Batch" or "SlowQuery"
}

// Make sure ProcessorNotFoundError implements the error interface
func (e ProcessorNotFoundError) Error() string {
	return fmt.Sprintf("no %sProcessor registered for app %s and op %s",
		e.ProcessorType, e.App, e.Op)
}

// Allow unwrapping to get the base error
func (e ProcessorNotFoundError) Unwrap() error {
	return ErrProcessorNotFound
}

const ALYA_BATCHCHUNK_NROWS = 10
const ALYA_BATCHSTATUS_CACHEDUR_SEC = 60

// Assuming global variables are defined elsewhere
// make all the maps sync maps to make them thread safe
var (
	mu     sync.Mutex // Ensures thread-safe access to the initfuncs map
	doneBy pgtype.Text
)

// JobManager is the main struct that manages the processing of batch jobs and slow queries.
// It is responsible for fetching jobs from the database, processing them using the registered processors.
// Life cycle of a batch job or slow query is as follows:
// 1. Fetch a block of rows from the database
// 2. Process each row in the block
// 3. Update the corresponding batchrows and batches records with the results
// 4. Check for completed batches and summarize them
type JobManager struct {
	Db                      *pgxpool.Pool
	Queries                 batchsqlc.Querier
	RedisClient             *redis.Client
	ObjStore                objstore.ObjectStore
	initblocks              map[string]InitBlock
	initfuncs               map[string]Initializer
	slowqueryprocessorfuncs map[string]SlowQueryProcessor
	batchprocessorfuncs     map[string]BatchProcessor
	Logger                  *logharbour.Logger
	Config                  JobManagerConfig
}

// NewJobManager creates a new instance of JobManager.
// It initializes the necessary fields and returns a pointer to the JobManager.
func NewJobManager(db *pgxpool.Pool, redisClient *redis.Client, minioClient *minio.Client, logger *logharbour.Logger, config *JobManagerConfig) *JobManager {
	if logger == nil {
		panic("logger cannot be nil")
	}
	if config == nil {
		config = &JobManagerConfig{}
	}
	// check zero value of each field in config and set their default values
	if config.BatchChunkNRows == 0 {
		config.BatchChunkNRows = ALYA_BATCHCHUNK_NROWS
	}
	if config.BatchStatusCacheDurSec == 0 {
		config.BatchStatusCacheDurSec = ALYA_BATCHSTATUS_CACHEDUR_SEC
	}
	if config.BatchOutputBucket == "" {
		config.BatchOutputBucket = "alya-batch-output"
		logger.Warn().LogActivity("No BatchOutputBucket configured, using default: alya-batch-output", nil)
	}

	return &JobManager{
		Db:                      db,
		Queries:                 batchsqlc.New(db),
		RedisClient:             redisClient,
		ObjStore:                objstore.NewMinioObjectStore(minioClient),
		initblocks:              make(map[string]InitBlock),
		initfuncs:               make(map[string]Initializer),
		slowqueryprocessorfuncs: make(map[string]SlowQueryProcessor),
		batchprocessorfuncs:     make(map[string]BatchProcessor),
		Logger:                  logger,
		Config:                  *config,
	}
}

var ErrInitializerAlreadyRegistered = errors.New("initializer already registered for this app")

// RegisterInitializer registers an initializer for a specific application.
// The initializer is responsible for initializing any required resources or state
// needed for processing batches or slow queries for that application.
//
// The initializer will be called by Alya to create an InitBlock instance that
// can be used by the processing functions (BatchProcessor or SlowQueryProcessor)
// to access any necessary resources or configuration for the application.
//
// Applications must register an initializer before registering any batch processor or
// slow query processor. It allows for proper initialization and
// cleanup of resources used by the processing functions.
func (jm *JobManager) RegisterInitializer(app string, initializer Initializer) error {
	mu.Lock()
	defer mu.Unlock()

	// Check if an initializer for this app already exists to prevent accidental overwrites
	if _, exists := jm.initfuncs[app]; exists {
		return fmt.Errorf("%w: app=%s", ErrInitializerAlreadyRegistered, app)
	}

	// Register the initializer for the app
	jm.initfuncs[app] = initializer
	return nil
}

// getOrCreateInitBlock retrieves an existing InitBlock for the given app, or creates a new one
// if it doesn't exist. It ensures thread-safe access to the initblocks map using a mutex lock.
func (jm *JobManager) getOrCreateInitBlock(app string) (InitBlock, error) {
	mu.Lock()
	defer mu.Unlock()

	// Check if an InitBlock already exists for this app
	initBlock, exists := jm.initblocks[app]
	if exists && initBlock != nil {
		return initBlock, nil
	}

	// No InitBlock exists for this app, so we need to create one
	// First, check if an initializer is registered for this app
	initializer, exists := jm.initfuncs[app]
	if !exists {
		return nil, &ConfigurationError{
			BaseErr: ErrInitializerNotFound,
			App:     app,
			Details: "no initializer registered",
		}
	}

	// Call the initializer to create a new InitBlock
	initBlock, err := initializer.Init(app)
	if err != nil {
		return nil, &ConfigurationError{
			BaseErr: ErrInitializerFailed,
			App:     app,
			Details: fmt.Sprintf("initialization failed: %v", err),
		}
	}

	// Cache the InitBlock for future use
	jm.initblocks[app] = initBlock
	return initBlock, nil
}

// Run is the main loop of the JobManager. It continuously fetches a block of rows from the database,
// processes each row either as a slow query or a batch job. After processing a block, it checks for
// completed batches and summarizes them.
// This method should be called in a separate goroutine. It is thread safe -- updates to database and Redis
// are executed atomically.
func (jm *JobManager) Run() {
	for {
		// Run a single iteration of the job processing loop
		// Only sleep if no rows were processed
		hadRows := jm.RunOneIteration()

		// Sleep only if no rows were found to process
		if !hadRows {
			time.Sleep(getRandomSleepDuration())
		}
	}
}

// RunWithContext is the context-aware version of the Run method. It performs the same operations
// as Run but respects context cancellation for clean shutdown. When the provided context is
// canceled, the method will exit cleanly.
// This should be preferred over Run() in production environments to allow for graceful shutdown.
func (jm *JobManager) RunWithContext(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Context was canceled, exit cleanly
			return
		default:
			// Run a single iteration of the job processing loop
			hadRows := jm.RunOneIterationWithContext(ctx)

			// Sleep only if no rows were found to process
			if !hadRows {
				select {
				case <-ctx.Done():
					return
				case <-time.After(getRandomSleepDuration()):
					// Continue to the next iteration
				}
			}
		}
	}
}

// RunOneIteration executes a single iteration of the job processing loop.
// It fetches a block of rows, processes them, and summarizes completed batches.
// This method is useful both for the main Run loop and for testing.
// Returns true if rows were processed, false otherwise.
func (jm *JobManager) RunOneIteration() bool {
	ctx := context.Background()

	// Use the context-aware version with a background context
	return jm.RunOneIterationWithContext(ctx)
}

// RunOneIterationWithContext executes a single iteration of the job processing loop,
// respecting the provided context for cancellation. If the context is canceled during
// execution, the method will attempt to clean up and exit as soon as possible.
// Returns true if rows were processed, false otherwise.
func (jm *JobManager) RunOneIterationWithContext(ctx context.Context) bool {
	// CANCELLATION POINT 1: Early check before starting any work
	// Prevents starting any operations if shutdown is already requested
	if ctx.Err() != nil {
		return false
	}

	// Begin a transaction
	jm.Logger.Debug0().LogActivity("Starting transaction for row status updates", nil)
	tx, err := jm.Db.Begin(ctx)
	if err != nil {
		jm.Logger.Error(err).LogActivity("Error starting transaction", nil)
		return false
	}

	// Create a new Queries instance using the transaction
	txQueries := batchsqlc.New(tx)

	// Fetch a block of rows from the database
	jm.Logger.Debug0().LogActivity("Fetching block of rows", map[string]any{
		"status": "queued",
		"limit": jm.Config.BatchChunkNRows,
	})
	blockOfRows, err := txQueries.FetchBlockOfRows(ctx, batchsqlc.FetchBlockOfRowsParams{
		Status: batchsqlc.StatusEnumQueued,
		Limit:  int32(jm.Config.BatchChunkNRows),
	})
	if err != nil {
		jm.Logger.Error(err).LogActivity("Error fetching block of rows", nil)
		tx.Rollback(ctx)
		return false
	}

	// If no rows are found, rollback and return false
	if len(blockOfRows) == 0 {
		jm.Logger.Debug0().LogActivity("No rows found in queue, rolling back transaction", nil)
		if err := tx.Rollback(ctx); err != nil {
			jm.Logger.Error(err).LogActivity("Error rolling back transaction", nil)
		}
		return false
	}
	
	jm.Logger.Info().LogActivity("Processing batch rows", map[string]any{
		"rowCount": len(blockOfRows),
	})

	// Process each row in the block
	for _, row := range blockOfRows {
		// CANCELLATION POINT 2: Before updating row statuses
		// Prevents unnecessary database updates and rolls back the transaction
		// Important check in loops to bail out early before each database operation
		if ctx.Err() != nil {
			tx.Rollback(ctx)
			return false
		}

		// Update the status of the batch row to "inprog"
		jm.Logger.Debug0().LogActivity("Updating batch row status to inprog", map[string]any{
			"rowId": row.Rowid,
			"batchId": row.Batch.String(),
			"app": row.App,
			"op": row.Op,
			"line": row.Line,
		})
		err := txQueries.UpdateBatchRowStatus(ctx, batchsqlc.UpdateBatchRowStatusParams{
			Rowid:  row.Rowid,
			Status: batchsqlc.StatusEnumInprog,
		})
		if err != nil {
			jm.Logger.Error(err).LogActivity("Error updating batch row status", map[string]any{
				"rowId": row.Rowid,
				"batchId": row.Batch.String(),
			})
			tx.Rollback(ctx)
			return false
		}

		if row.Status == batchsqlc.StatusEnumQueued {
			// Update the status of the batch to "inprog"
			// Log the status change
			changeDetails := logharbour.ChangeInfo{
				Entity: "BatchRow",
				Op:     "StatusUpdated",
				Changes: []logharbour.ChangeDetail{
					{"status", batchsqlc.StatusEnumQueued, batchsqlc.StatusEnumInprog},
				},
			}
			jm.Logger.LogDataChange("Batch row status updated to inprog", changeDetails)

			jm.Logger.Debug0().LogActivity("Updating batch status to inprog", map[string]any{
				"batchId": row.Batch.String(),
				"app": row.App,
				"op": row.Op,
			})
			err := txQueries.UpdateBatchStatus(ctx, batchsqlc.UpdateBatchStatusParams{
				ID:     row.Batch,
				Status: batchsqlc.StatusEnumInprog,
			})
			if err != nil {
				jm.Logger.Error(err).LogActivity("Error updating batch status", map[string]any{
					"batchId": row.Batch.String(),
				})
				jm.Logger.Debug0().LogActivity("Rolling back transaction due to batch status update error", nil)
				if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
					jm.Logger.Error(rollbackErr).LogActivity("Error rolling back transaction", nil)
				}
				return false
			}
		}
	}

	// CANCELLATION POINT 3: Before committing first transaction
	// Avoids committing changes if shutdown was requested during row status updates
	// Ensures database consistency by rolling back incomplete work
	if ctx.Err() != nil {
		tx.Rollback(ctx)
		return false
	}

	jm.Logger.Debug0().LogActivity("Committing transaction for status updates", map[string]any{
		"rowCount": len(blockOfRows),
	})
	err = tx.Commit(ctx)
	if err != nil {
		jm.Logger.Error(err).LogActivity("Error committing transaction", nil)
		return false
	}

	// Process the rows
	// Track which batch+app+op combinations we've already processed for errors
	processedErrorCombinations := make(map[string]bool)

	for _, row := range blockOfRows {
		// CANCELLATION POINT 4: Before processing each row
		// Prevents starting potentially long-running processor functions
		// Important because row processing happens outside a transaction
		// Allows fast response to cancellation during CPU-intensive operations
		if ctx.Err() != nil {
			return false
		}

		// send queries instance, not transaction
		q := jm.Queries
		jm.Logger.Debug0().LogActivity("Processing row", map[string]any{
			"rowId": row.Rowid,
			"batchId": row.Batch.String(),
			"app": row.App,
			"op": row.Op,
			"line": row.Line,
		})
		_, err = jm.processRow(q, row)
		if err != nil {
			jm.Logger.Error(err).LogActivity("Error processing row", map[string]any{
				"rowId": row.Rowid,
				"batchId": row.Batch.String(),
				"app": row.App,
				"op": row.Op,
			})
			// Apply appropriate error handling strategy based on error type
			jm.handleProcessingError(err, row, processedErrorCombinations)
			continue
		}
	}

	// CANCELLATION POINT 5: Before starting summary transaction
	// Avoids starting a new transaction phase if shutdown is requested
	// Batch summarization is a separate logical phase that can be skipped
	// Prevents unnecessary work after the main processing is complete
	if ctx.Err() != nil {
		return true // Return true because we did process rows
	}

	// create a new transaction for the summarizeCompletedBatches
	jm.Logger.Debug0().LogActivity("Starting transaction for batch summarization", nil)
	tx, err = jm.Db.Begin(ctx)
	if err != nil {
		jm.Logger.Error(err).LogActivity("Error starting transaction for summarization", nil)
		return true // Return true because we did process rows
	}

	txQueries = batchsqlc.New(tx)

	// Create a map to store unique batch IDs
	batchSet := make(map[uuid.UUID]bool)
	for _, row := range blockOfRows {
		batchSet[row.Batch] = true
	}

	// Check for completed batches and summarize them
	jm.Logger.Debug0().LogActivity("Checking for completed batches", map[string]any{
		"batchCount": len(batchSet),
	})
	if err := jm.summarizeCompletedBatches(txQueries, batchSet); err != nil {
		jm.Logger.Error(err).LogActivity("Error summarizing completed batches", nil)
		tx.Rollback(ctx)
		return true // Return true because we did process rows
	}

	// CANCELLATION POINT 6: Before committing summary transaction
	// Ensures summary changes aren't committed if shutdown was requested
	// Maintains database consistency during shutdown
	// Last chance to roll back before making summarization changes permanent
	if ctx.Err() != nil {
		tx.Rollback(ctx)
		return true // Return true because we did process rows
	}

	jm.Logger.Debug0().LogActivity("Committing transaction for batch summarization", nil)
	err = tx.Commit(ctx)
	if err != nil {
		jm.Logger.Error(err).LogActivity("Error committing summarization transaction", nil)
		return false
	}

	// Close and clean up initblocks
	jm.closeInitBlocks()
	
	// Return true to indicate rows were processed
	return true
}

func (jm *JobManager) processRow(txQueries batchsqlc.Querier, row batchsqlc.FetchBlockOfRowsRow) (batchsqlc.StatusEnum, error) {
	// Process the row based on its type (slow query or batch job)
	if row.Line == 0 {
		jm.Logger.Debug0().LogActivity("Processing slow query", map[string]any{
			"rowId": row.Rowid,
			"batchId": row.Batch.String(),
			"app": row.App,
			"op": row.Op,
		})
		return jm.processSlowQuery(txQueries, row)
	} else {
		jm.Logger.Debug0().LogActivity("Processing batch job", map[string]any{
			"rowId": row.Rowid,
			"batchId": row.Batch.String(),
			"app": row.App,
			"op": row.Op,
			"line": row.Line,
		})
		return jm.processBatchJob(txQueries, row)
	}
}

// processSlowQuery processes a single slow query job. It retrieves the registered SlowQueryProcessor
// for the given app and op, fetches the associated InitBlock, and invokes the processor's DoSlowQuery
// method. It then calls updateSlowQueryResult to update the corresponding batchrows and batches records
// with the processing results. If the processor is not found or the processing fails, an error is returned.

func (jm *JobManager) processSlowQuery(txQueries batchsqlc.Querier, row batchsqlc.FetchBlockOfRowsRow) (batchsqlc.StatusEnum, error) {
	jm.Logger.Info().LogActivity("Starting slow query processing", map[string]any{
		"rowId": row.Rowid,
		"batchId": row.Batch.String(),
		"app": row.App,
		"op": row.Op,
	})
	// Retrieve the SlowQueryProcessor for the app and op
	p, exists := jm.slowqueryprocessorfuncs[string(row.App)+row.Op]
	if !exists {
		return batchsqlc.StatusEnumFailed, &ProcessorNotFoundError{
			App:           string(row.App),
			Op:            row.Op,
			ProcessorType: "SlowQuery",
		}
	}

	// Assert that the processor is of the correct type
	processor, ok := p.(SlowQueryProcessor)
	if !ok {
		return batchsqlc.StatusEnumFailed, &ConfigurationError{
			BaseErr: ErrInvalidProcessorType,
			App:     string(row.App),
			Op:      row.Op,
			Details: "registered processor does not implement SlowQueryProcessor interface",
		}
	}

	// Get or create the initblock for the app
	initBlock, err := jm.getOrCreateInitBlock(string(row.App))
	if err != nil {
		jm.Logger.Error(err).LogActivity("Error getting or creating initblock", map[string]any{
			"app": row.App,
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, err
	}

	// Process the slow query using the registered processor
	rowContext, err := NewJSONstr(string(row.Context))
	if err != nil {
		jm.Logger.Error(err).LogActivity("Error parsing row context for slow query", map[string]any{
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}
	rowInput, err := NewJSONstr(string(row.Input))
	if err != nil {
		jm.Logger.Error(err).LogActivity("Error parsing row input for slow query", map[string]any{
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}
	
	jm.Logger.Debug0().LogActivity("Executing slow query processor", map[string]any{
		"rowId": row.Rowid,
		"processor": fmt.Sprintf("%T", processor),
	})
	status, result, messages, outputFiles, err := processor.DoSlowQuery(initBlock, rowContext, rowInput)
	if err != nil {
		jm.Logger.Error(err).LogActivity("Slow query processor failed", map[string]any{
			"rowId": row.Rowid,
			"app": row.App,
			"op": row.Op,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}
	
	jm.Logger.Info().LogActivity("Slow query processor completed", map[string]any{
		"rowId": row.Rowid,
		"status": status,
		"hasMessages": len(messages) > 0,
		"hasOutputFiles": len(outputFiles) > 0,
	})

	// Update the corresponding batchrows and batches records with the results
	jm.Logger.Debug0().LogActivity("Updating slow query result", map[string]any{
		"rowId": row.Rowid,
		"status": status,
	})
	if err := updateSlowQueryResult(txQueries, row, status, result, messages, outputFiles); err != nil {
		jm.Logger.Error(err).LogActivity("Error updating slow query result", map[string]any{
			"rowId": row.Rowid,
			"app": row.App,
			"op": row.Op,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error updating slow query result for app %s and op %s: %v", row.App, row.Op, err)
	}

	jm.Logger.Info().LogActivity("Slow query completed successfully", map[string]any{
		"rowId": row.Rowid,
		"batchId": row.Batch.String(),
		"status": status,
	})
	return status, nil
}

// processBatchJob processes a single batch job. It retrieves the registered BatchProcessor for the
// given app and op, fetches the associated InitBlock, and invokes the processor's DoBatchJob method.
// It then calls updateBatchJobResult to update the corresponding batchrows record with the processing results.
// If the processor is not found or the processing fails, an error is returned.
func (jm *JobManager) processBatchJob(txQueries batchsqlc.Querier, row batchsqlc.FetchBlockOfRowsRow) (batchsqlc.StatusEnum, error) {
	jm.Logger.Info().LogActivity("Starting batch job processing", map[string]any{
		"rowId": row.Rowid,
		"batchId": row.Batch.String(),
		"app": row.App,
		"op": row.Op,
		"line": row.Line,
	})
	// Retrieve the BatchProcessor for the app and op
	p, exists := jm.batchprocessorfuncs[string(row.App)+row.Op]
	if !exists {
		jm.Logger.Warn().LogActivity("Batch processor not found", map[string]any{
			"app": row.App,
			"op": row.Op,
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, &ProcessorNotFoundError{
			App:           string(row.App),
			Op:            row.Op,
			ProcessorType: "Batch",
		}
	}

	// Assert that the processor is of the correct type
	processor, ok := p.(BatchProcessor)
	if !ok {
		return batchsqlc.StatusEnumFailed, &ConfigurationError{
			BaseErr: ErrInvalidProcessorType,
			App:     string(row.App),
			Op:      row.Op,
			Details: "registered processor does not implement BatchProcessor interface",
		}
	}

	// Get or create the initblock for the app
	initBlock, err := jm.getOrCreateInitBlock(string(row.App))
	if err != nil {
		jm.Logger.Error(err).LogActivity("Error getting or creating initblock", map[string]any{
			"app": row.App,
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, err
	}

	// Process the batch job using the registered processor
	rowContext, err := NewJSONstr(string(row.Context))
	if err != nil {
		jm.Logger.Error(err).LogActivity("Error parsing row context for batch job", map[string]any{
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing batch job for app %s and op %s: %v", row.App, row.Op, err)
	}
	rowInput, err := NewJSONstr(string(row.Input))
	if err != nil {
		jm.Logger.Error(err).LogActivity("Error parsing row input for batch job", map[string]any{
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing batch job for app %s and op %s: %v", row.App, row.Op, err)
	}
	
	jm.Logger.Debug0().LogActivity("Executing batch processor", map[string]any{
		"rowId": row.Rowid,
		"processor": fmt.Sprintf("%T", processor),
		"line": row.Line,
	})
	status, result, messages, blobRows, err := processor.DoBatchJob(initBlock, rowContext, int(row.Line), rowInput)
	if err != nil {
		jm.Logger.Error(err).LogActivity("Batch processor failed", map[string]any{
			"rowId": row.Rowid,
			"app": row.App,
			"op": row.Op,
			"line": row.Line,
		})
	}
	
	jm.Logger.Info().LogActivity("Batch processor completed", map[string]any{
		"rowId": row.Rowid,
		"status": status,
		"hasMessages": len(messages) > 0,
		"hasBlobRows": len(blobRows) > 0,
		"error": err != nil,
	})
	// TODO: check if it should return the error and what its effects are
	// if err != nil {
	// return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing batch job for app %s and op %s: %v", row.App, row.Op, err)
	// }

	// Update the corresponding batchrows record with the results
	jm.Logger.Debug0().LogActivity("Updating batch job result", map[string]any{
		"rowId": row.Rowid,
		"status": status,
	})
	if err := jm.updateBatchJobResult(txQueries, row, status, result, messages, blobRows); err != nil {
		jm.Logger.Error(err).LogActivity("Error updating batch job result", map[string]any{
			"rowId": row.Rowid,
			"app": row.App,
			"op": row.Op,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error updating batch job result for app %s and op %s: %v", row.App, row.Op, err)
	}

	jm.Logger.Info().LogActivity("Batch job completed successfully", map[string]any{
		"rowId": row.Rowid,
		"batchId": row.Batch.String(),
		"status": status,
		"line": row.Line,
	})
	return status, nil
}

// updateSlowQueryResult updates the batchrows and batches records with the results of a processed
// slow query.
// This function is called after a slow query has been processed by the registered SlowQueryProcessor.
func updateSlowQueryResult(txQueries batchsqlc.Querier, row batchsqlc.FetchBlockOfRowsRow, status batchsqlc.StatusEnum, result JSONstr, messages []wscutils.ErrorMessage, outputFiles map[string]string) error {
	// Marshal messages to JSON
	var messagesJSON, outputFilesJSON []byte
	if len(messages) > 0 {
		var err error
		messagesJSON, err = json.Marshal(messages)
		if err != nil {
			return fmt.Errorf("failed to marshal messages to JSON: %v", err)
		}
	}

	// Update the batchrows record with the results
	err := txQueries.UpdateBatchRowsSlowQuery(context.Background(), batchsqlc.UpdateBatchRowsSlowQueryParams{
		Rowid:    int64(row.Rowid),
		Status:   batchsqlc.StatusEnum(status),
		Doneat:   pgtype.Timestamp{Time: time.Now(), Valid: true},
		Res:      []byte(result.String()),
		Messages: messagesJSON,
		Doneby:   doneBy,
	})
	if err != nil {
		return err
	}
	// Marshal outputFiles to JSON
	outputFilesJSON, err = json.Marshal(outputFiles)
	if err != nil {
		return fmt.Errorf("failed to marshal outputFiles to JSON: %v", err)
	}

	// Update the batches record with the result
	err = txQueries.UpdateBatchResult(context.Background(), batchsqlc.UpdateBatchResultParams{
		Outputfiles: outputFilesJSON,
		Status:      batchsqlc.StatusEnum(status),
		Doneat:      pgtype.Timestamp{Time: time.Now(), Valid: true},
		ID:          row.Batch,
	})
	if err != nil {
		return err
	}

	return nil
}

// updateBatchJobResult updates the batchrows record with the results of a processed batch job.
// This function is called after a batch job has been processed by the registered BatchProcessor.
func (jm *JobManager) updateBatchJobResult(txQueries batchsqlc.Querier, row batchsqlc.FetchBlockOfRowsRow, status batchsqlc.StatusEnum, result JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string) error {
	// Marshal messages to JSON
	var messagesJSON []byte
	if len(messages) > 0 {
		var err error
		messagesJSON, err = json.Marshal(messages)
		if err != nil {
			return fmt.Errorf("failed to marshal messages to JSON: %v", err)
		}
	}

	// Marshal blobRows to JSON
	var blobRowsJSON []byte
	if len(blobRows) > 0 {
		var err error
		blobRowsJSON, err = json.Marshal(blobRows)
		if err != nil {
			return fmt.Errorf("failed to marshal blobRows to JSON: %v", err)
		}
	}

	// Update the batchrows record with the results
	jm.Logger.LogDataChange("Batch row updated", logharbour.ChangeInfo{
		Entity: "BatchRow",
		Op:     "Update",
		Changes: []logharbour.ChangeDetail{
			{"status", row.Status, batchsqlc.StatusEnum(status)},
		},
	})
	err := txQueries.UpdateBatchRowsBatchJob(context.Background(), batchsqlc.UpdateBatchRowsBatchJobParams{
		Rowid:    int64(row.Rowid),
		Status:   batchsqlc.StatusEnum(status),
		Doneat:   pgtype.Timestamp{Time: time.Now(), Valid: true},
		Res:      []byte(result.String()),
		Blobrows: blobRowsJSON,
		Messages: messagesJSON,
		Doneby:   doneBy,
	})
	if err != nil {
		return err
	}

	return nil
}

func (jm *JobManager) summarizeCompletedBatches(q *batchsqlc.Queries, batchSet map[uuid.UUID]bool) error {
	jm.Logger.Debug0().LogActivity("Summarizing completed batches", map[string]any{
		"batchCount": len(batchSet),
	})
	for batchID := range batchSet {
		jm.Logger.Debug0().LogActivity("Summarizing batch", map[string]any{
			"batchId": batchID.String(),
		})
		if err := jm.summarizeBatch(q, batchID); err != nil {
			jm.Logger.Error(err).LogActivity("Error summarizing batch", map[string]any{
				"batchId": batchID.String(),
			})
			continue
		}
	}

	return nil
}

func (jm *JobManager) closeInitBlocks() {
	for app, initBlock := range jm.initblocks {
		if initBlock != nil {
			if closer, ok := initBlock.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					jm.Logger.Error(err).LogActivity("Error closing initblock", map[string]any{
						"app": app,
					})
				}
			}
		}
	}
	jm.initblocks = make(map[string]InitBlock)
}

func getRandomSleepDuration() time.Duration {
	// Generate a random sleep duration between 30 and 60 seconds
	return time.Duration(rand.Intn(31)+30) * time.Second
}

// updateRowStatusToFailed updates a single row's status to failed with the given error information.
func (jm *JobManager) updateRowStatusToFailed(rowid int64, errCode string, msgID int, errMsg string) error {
	errorMessages := []wscutils.ErrorMessage{
		{
			MsgID:   msgID,
			ErrCode: errCode,
			Vals:    []string{errMsg}, // Include the detailed error message in Vals
			// Field is optional and can be left empty
		},
	}

	messagesJSON, err := json.Marshal(errorMessages)
	if err != nil {
		return fmt.Errorf("failed to marshal error messages: %v", err)
	}

	// Update the batch row using the BatchJob update function (since it includes message field)
	err = jm.Queries.UpdateBatchRowsBatchJob(context.Background(), batchsqlc.UpdateBatchRowsBatchJobParams{
		Rowid:    rowid,
		Status:   batchsqlc.StatusEnumFailed,
		Doneat:   pgtype.Timestamp{Time: time.Now(), Valid: true},
		Res:      []byte("{}"), // Empty result
		Messages: messagesJSON,
		Blobrows: nil,
		Doneby:   doneBy,
	})

	return err
}

// markAllRowsWithSameAppOpAsFailed marks all rows in the same batch with the same app+op as failed.
func (jm *JobManager) markAllRowsWithSameAppOpAsFailed(batchID uuid.UUID, app, op, errCode string, msgID int, errMsg string) error {
	// Prepare error message JSON
	errorMessages := []wscutils.ErrorMessage{
		{
			MsgID:   msgID,
			ErrCode: errCode,
			Vals:    []string{errMsg}, // Include the detailed error message in Vals
			// Field is optional and can be left empty
		},
	}
	messagesJSON, err := json.Marshal(errorMessages)
	if err != nil {
		return fmt.Errorf("failed to marshal error messages: %v", err)
	}

	// Update all rows in the batch with this app+op that are not already completed
	// using the sqlc-generated function
	err = jm.Queries.UpdateBatchRowsByBatchAppOp(context.Background(), batchsqlc.UpdateBatchRowsByBatchAppOpParams{
		Status:   batchsqlc.StatusEnumFailed,
		Doneat:   pgtype.Timestamp{Time: time.Now(), Valid: true},
		Messages: messagesJSON,
		ID:       batchID,
		App:      app,
		Op:       op,
	})

	return err
}

// updateBatchStatusToFailed updates a batch's status to failed.
func (jm *JobManager) updateBatchStatusToFailed(batchID uuid.UUID) error {
	// Update the batch status to failed
	err := jm.Queries.UpdateBatchStatus(context.Background(), batchsqlc.UpdateBatchStatusParams{
		ID:     batchID,
		Status: batchsqlc.StatusEnumFailed,
		Doneat: pgtype.Timestamp{Time: time.Now(), Valid: true},
	})

	if err == nil {
		// Also update Redis cache if applicable
		if jm.RedisClient != nil {
			cacheErr := updateStatusInRedis(jm.RedisClient, batchID, batchsqlc.StatusEnumFailed,
				jm.Config.BatchStatusCacheDurSec)
			if cacheErr != nil {
				jm.Logger.Warn().LogActivity("Failed to update Redis cache for batch", map[string]any{
					"batchId": batchID.String(),
					"error": cacheErr.Error(),
				})
			}
		}
	}

	return err
}

// handleProcessingError handles various error scenarios that may occur during job processing.
// It categorizes errors and applies the appropriate handling strategy based on the error type.
func (jm *JobManager) handleProcessingError(err error, row batchsqlc.FetchBlockOfRowsRow, processedBatches map[string]bool) {
	batchID := row.Batch
	app := string(row.App)
	op := row.Op

	// Log the start of error handling with context
	jm.Logger.Info().LogActivity("Starting error handling for job", map[string]any{
		"batchID": batchID.String(),
		"app":     app,
		"op":      op,
		"rowID":   row.Rowid,
	})

	// Handle different error types
	switch {
	case isConfigurationError(err):
		// Configuration errors affect all rows with the same app or app+op
		// These include missing processors, invalid processor types, or initializer issues

		// Create a unique key based on the error type
		var key string
		var errCode string
		var msgID int

		// Determine the scope of the error (app-wide or app+op)
		if errors.Is(err, ErrProcessorNotFound) || errors.Is(err, ErrInvalidProcessorType) {
			// These errors affect all rows with the same app+op
			key = fmt.Sprintf("%s|%s|%s", batchID.String(), app, op)

			if errors.Is(err, ErrProcessorNotFound) {
				errCode = ErrCodeConfiguration
				msgID = MsgIDProcessorNotFound
			} else {
				errCode = ErrCodeConfiguration
				msgID = MsgIDInvalidProcessorType
			}

			// Check if we've already handled this batch+app+op combination
			if processedBatches[key] {
				return
			}
			processedBatches[key] = true

			// Mark all other rows with the same batch+app+op as failed
			if failErr := jm.markAllRowsWithSameAppOpAsFailed(batchID, app, op, errCode, msgID, err.Error()); failErr != nil {
				jm.Logger.Error(failErr).LogActivity("Error marking all rows with same batch+app+op as failed", map[string]any{
					"batchID": batchID.String(),
					"app":     app,
					"op":      op,
					"errCode": errCode,
				})
			}
		} else if errors.Is(err, ErrInitializerNotFound) || errors.Is(err, ErrInitializerFailed) {
			// These errors affect all rows with the same app
			key = fmt.Sprintf("%s|%s", batchID.String(), app)

			if errors.Is(err, ErrInitializerNotFound) {
				errCode = ErrCodeConfiguration
				msgID = MsgIDInitializerNotFound
			} else {
				errCode = ErrCodeConfiguration
				msgID = MsgIDInitializerFailed
			}

			// Check if we've already handled this batch+app combination
			if processedBatches[key] {
				return
			}
			processedBatches[key] = true

			// Mark all rows with the same batch+app as failed
			if failErr := jm.markAllRowsWithSameAppAsFailed(batchID, app, errCode, msgID, err.Error()); failErr != nil {
				jm.Logger.Error(failErr).LogActivity("Error marking all rows with same batch+app as failed", map[string]any{
					"batchID": batchID.String(),
					"app":     app,
					"errCode": errCode,
				})
			}
		} else {
			// Unknown configuration error, just handle the current row
			errCode = ErrCodeConfiguration
			msgID = MsgIDUnknownConfigurationError
			// Update the specific row status to failed
			if rowErr := jm.updateRowStatusToFailed(row.Rowid, errCode, msgID, err.Error()); rowErr != nil {
				jm.Logger.Error(rowErr).LogActivity("Error updating row status to failed", map[string]any{
					"rowID":   row.Rowid,
					"batchID": batchID.String(),
					"errCode": errCode,
				})
			}
		}

		// Mark the current row as failed
		if failErr := jm.updateRowStatusToFailed(row.Rowid, errCode, msgID, err.Error()); failErr != nil {
			jm.Logger.Error(failErr).LogActivity("Error updating row status to failed", map[string]any{
				"rowID":   row.Rowid,
				"batchID": batchID.String(),
				"errCode": errCode,
			})
		}

		// Update the batch status to failed
		if failErr := jm.updateBatchStatusToFailed(batchID); failErr != nil {
			jm.Logger.Error(failErr).LogActivity("Error updating batch status to failed", map[string]any{
				"batchID": batchID.String(),
				"errCode": errCode,
			})
		} else {
			jm.Logger.Info().LogActivity("Batch marked as failed due to configuration error", map[string]any{
				"batchID": batchID.String(),
				"app":     app,
				"op":      op,
				"error":   err.Error(),
				"errCode": errCode,
			})
		}

	default:
		// For other errors, we just mark the individual row as failed
		// These are typically transient or data-specific errors
		errCode := ErrCodeProcessing
		msgID := MsgIDProcessingError

		// Mark just this row as failed
		if failErr := jm.updateRowStatusToFailed(row.Rowid, errCode, msgID, err.Error()); failErr != nil {
			jm.Logger.Error(failErr).LogActivity("Error updating row status to failed", map[string]any{
				"rowID":   row.Rowid,
				"batchID": batchID.String(),
				"errCode": errCode,
			})
		} else {
			jm.Logger.Info().LogActivity("Row marked as failed due to processing error", map[string]any{
				"rowID":   row.Rowid,
				"batchID": batchID.String(),
				"app":     app,
				"op":      op,
				"error":   err.Error(),
			})
		}
	}
}

// isConfigurationError checks if an error is a configuration-related error
func isConfigurationError(err error) bool {
	// Check for errors.Is matches
	if errors.Is(err, ErrProcessorNotFound) ||
		errors.Is(err, ErrInitializerNotFound) ||
		errors.Is(err, ErrInitializerFailed) ||
		errors.Is(err, ErrInvalidProcessorType) {
		return true
	}

	// Check for errors.As matches
	var configErr ConfigurationError
	var procNotFoundErr ProcessorNotFoundError
	return errors.As(err, &configErr) || errors.As(err, &procNotFoundErr)
}

// markAllRowsWithSameAppAsFailed marks all rows in the same batch with the same app as failed
func (jm *JobManager) markAllRowsWithSameAppAsFailed(batchID uuid.UUID, app, errCode string, msgID int, errMsg string) error {
	// Prepare error message JSON
	errorMessages := []wscutils.ErrorMessage{
		{
			MsgID:   msgID,
			ErrCode: errCode,
			Vals:    []string{errMsg}, // Include the detailed error message in Vals
			// Field is optional and can be left empty
		},
	}
	messagesJSON, err := json.Marshal(errorMessages)
	if err != nil {
		return fmt.Errorf("failed to marshal error messages: %v", err)
	}

	// Update all rows in the batch with this app that are not already completed
	// using the sqlc-generated function
	err = jm.Queries.UpdateBatchRowsByBatchApp(context.Background(), batchsqlc.UpdateBatchRowsByBatchAppParams{
		Status:   batchsqlc.StatusEnumFailed,
		Doneat:   pgtype.Timestamp{Time: time.Now(), Valid: true},
		Messages: messagesJSON,
		ID:       batchID,
		App:      app,
	})

	return err
}
