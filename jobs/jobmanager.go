package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"runtime/debug"
	"strings"
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
	ErrBatchHasPendingRows    = errors.New("batch has pending rows")
	ErrBatchLockNotAcquired  = errors.New("batch lock held by another worker")
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
const ALYA_POLLING_INTERVAL_SEC = 45

// Assuming global variables are defined elsewhere
// make all the maps sync maps to make them thread safe
var (
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
	db                      *pgxpool.Pool
	queries                 batchsqlc.Querier
	redisClient             *redis.Client
	objStore                objstore.ObjectStore
	initblocks              map[string]InitBlock
	initfuncs               map[string]Initializer
	slowqueryprocessorfuncs map[string]SlowQueryProcessor
	batchprocessorfuncs     map[string]BatchProcessor
	logger                  *logharbour.Logger
	config                  JobManagerConfig
	mu                      sync.RWMutex // Protects initblocks and initfuncs maps
	instanceID              string       // Unique identifier for this JobManager instance
}

// generateInstanceID creates a unique identifier for a JobManager instance.
// Format: hostname-PID-timestamp (nanoseconds).
func generateInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().UnixNano())
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
	if config.PollingIntervalSec == 0 {
		config.PollingIntervalSec = ALYA_POLLING_INTERVAL_SEC
	}

	return &JobManager{
		db:                      db,
		queries:                 batchsqlc.New(db),
		redisClient:             redisClient,
		objStore:                objstore.NewMinioObjectStore(minioClient),
		initblocks:              make(map[string]InitBlock),
		initfuncs:               make(map[string]Initializer),
		slowqueryprocessorfuncs: make(map[string]SlowQueryProcessor),
		batchprocessorfuncs:     make(map[string]BatchProcessor),
		logger:                  logger,
		config:                  *config,
		instanceID:              generateInstanceID(),
	}
}

// InstanceID returns the unique identifier for this JobManager instance.
func (jm *JobManager) InstanceID() string {
	return jm.instanceID
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
	jm.mu.Lock()
	defer jm.mu.Unlock()

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
	// First try with read lock for better performance
	jm.logger.Debug0().LogActivity("Acquiring read lock for initblock lookup", map[string]any{
		"app": app,
	})
	jm.mu.RLock()
	initBlock, exists := jm.initblocks[app]
	jm.mu.RUnlock()
	
	if exists && initBlock != nil {
		jm.logger.Debug0().LogActivity("InitBlock found in cache", map[string]any{
			"app": app,
		})
		return initBlock, nil
	}

	// Need to create new InitBlock, so acquire write lock
	jm.logger.Debug0().LogActivity("Acquiring write lock to create initblock", map[string]any{
		"app": app,
	})
	jm.mu.Lock()
	defer jm.mu.Unlock()

	// Double-check if an InitBlock was created while we were waiting for the lock
	initBlock, exists = jm.initblocks[app]
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
	jm.logger.Debug0().LogActivity("Calling initializer.Init", map[string]any{
		"app": app,
		"initializer": fmt.Sprintf("%T", initializer),
	})
	initStartTime := time.Now()
	initBlock, err := initializer.Init(app)
	initElapsed := time.Since(initStartTime)
	if err != nil {
		jm.logger.Error(err).LogActivity("Initializer.Init failed", map[string]any{
			"app": app,
			"elapsedMs": initElapsed.Milliseconds(),
		})
		return nil, &ConfigurationError{
			BaseErr: ErrInitializerFailed,
			App:     app,
			Details: fmt.Sprintf("initialization failed: %v", err),
		}
	}
	jm.logger.Debug0().LogActivity("Initializer.Init completed", map[string]any{
		"app": app,
		"elapsedMs": initElapsed.Milliseconds(),
	})

	// Cache the InitBlock for future use
	jm.initblocks[app] = initBlock
	jm.logger.Debug0().LogActivity("InitBlock cached successfully", map[string]any{
		"app": app,
	})
	return initBlock, nil
}

// Run is the main loop of the JobManager. It continuously fetches a block of rows from the database,
// processes each row either as a slow query or a batch job. After processing a block, it checks for
// completed batches and summarizes them.
// This method should be called in a separate goroutine. It is thread safe -- updates to database and Redis
// are executed atomically.
func (jm *JobManager) Run() {
	ctx := context.Background()

	go jm.runHeartbeat()
	go jm.runPeriodicRecovery(ctx)
	go jm.runPeriodicSweep(ctx)

	// Circuit breaker pattern at the supervisor layer:
	// This is the ONLY layer where we make health decisions about the entire system.
	// We tolerate occasional panics (transient issues) but exit on repeated failures
	// (systemic issues). Lower layers MUST always recover to ensure cleanup.
	consecutivePanics := 0
	const maxConsecutivePanics = 3

	for {
		// Wrap each iteration in a function with panic recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					consecutivePanics++
					jm.logger.Error(fmt.Errorf("panic recovered: %v", r)).LogActivity("Panic in JobManager.Run", map[string]any{
						"panic": fmt.Sprintf("%v", r),
						"stackTrace": string(debug.Stack()),
						"consecutivePanics": consecutivePanics,
						"maxConsecutivePanics": maxConsecutivePanics,
					})
					
					// If too many consecutive panics, re-panic to crash the goroutine
					// This indicates a systemic issue that requires intervention
					if consecutivePanics >= maxConsecutivePanics {
						jm.logger.Error(nil).LogActivity("Circuit breaker triggered - too many consecutive panics", map[string]any{
							"consecutivePanics": consecutivePanics,
							"threshold": maxConsecutivePanics,
						})
						panic(fmt.Sprintf("JobManager circuit breaker: %d consecutive panics (threshold: %d), last panic: %v", 
							consecutivePanics, maxConsecutivePanics, r))
					}
				}
			}()

			// Run a single iteration of the job processing loop
			// Only sleep if no rows were processed
			hadRows := jm.RunOneIteration()
			
			// If we reach here, the iteration completed successfully
			// Reset the panic counter
			if consecutivePanics > 0 {
				jm.logger.Info().LogActivity("JobManager iteration succeeded, resetting panic counter", map[string]any{
					"previousConsecutivePanics": consecutivePanics,
				})
				consecutivePanics = 0
			}

			// Sleep only if no rows were found to process
			if !hadRows {
				time.Sleep(jm.getRandomSleepDuration())
			}
		}()
	}
}

// RunWithContext is the context-aware version of the Run method. It performs the same operations
// as Run but respects context cancellation for clean shutdown. When the provided context is
// canceled, the method will exit cleanly.
// This should be preferred over Run() in production environments to allow for graceful shutdown.
func (jm *JobManager) RunWithContext(ctx context.Context) {
	go jm.runHeartbeat()
	go jm.runPeriodicRecovery(ctx)
	go jm.runPeriodicSweep(ctx)

	// Circuit breaker pattern: same as Run() but respects context cancellation
	consecutivePanics := 0
	const maxConsecutivePanics = 3

	for {
		select {
		case <-ctx.Done():
			// Context was canceled, exit cleanly
			jm.logger.Info().LogActivity("JobManager exiting due to context cancellation", nil)
			return
		default:
			// Wrap each iteration with panic recovery and circuit breaker
			func() {
				defer func() {
					if r := recover(); r != nil {
						consecutivePanics++
						jm.logger.Error(fmt.Errorf("panic recovered: %v", r)).LogActivity("Panic in JobManager.RunWithContext", map[string]any{
							"panic": fmt.Sprintf("%v", r),
							"stackTrace": string(debug.Stack()),
							"consecutivePanics": consecutivePanics,
							"maxConsecutivePanics": maxConsecutivePanics,
						})
						
						// Circuit breaker: exit if too many consecutive panics
						if consecutivePanics >= maxConsecutivePanics {
							jm.logger.Error(nil).LogActivity("Circuit breaker triggered - too many consecutive panics", map[string]any{
								"consecutivePanics": consecutivePanics,
								"threshold": maxConsecutivePanics,
							})
							panic(fmt.Sprintf("JobManager circuit breaker: %d consecutive panics (threshold: %d), last panic: %v", 
								consecutivePanics, maxConsecutivePanics, r))
						}
					}
				}()

				// Run a single iteration of the job processing loop
				hadRows := jm.RunOneIterationWithContext(ctx)
				
				// Success - reset panic counter
				if consecutivePanics > 0 {
					jm.logger.Info().LogActivity("JobManager iteration succeeded, resetting panic counter", map[string]any{
						"previousConsecutivePanics": consecutivePanics,
					})
					consecutivePanics = 0
				}

				// Sleep only if no rows were found to process
				if !hadRows {
					select {
					case <-ctx.Done():
						return
					case <-time.After(jm.getRandomSleepDuration()):
						// Continue to the next iteration
					}
				}
			}()
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
func (jm *JobManager) RunOneIterationWithContext(ctx context.Context) (hadRows bool) {
	// CRITICAL: This layer MUST ALWAYS recover from panics. Here's why:
	// 1. We manage database transactions that MUST be rolled back on failure
	// 2. Panics here would leave rows locked with FOR UPDATE, causing permanent stuck batches
	// 3. Database connections would leak without proper cleanup
	// 
	// This is NOT the place to make health decisions - we just ensure cleanup and
	// let the supervisor layer (Run/RunWithContext) decide whether to exit.
	defer func() {
		if r := recover(); r != nil {
			jm.logger.Error(fmt.Errorf("panic recovered: %v", r)).LogActivity("Panic in RunOneIterationWithContext", map[string]any{
				"panic": fmt.Sprintf("%v", r),
				"stackTrace": string(debug.Stack()),
			})
			// Return false to indicate no successful processing
			hadRows = false
		}
	}()

	// CANCELLATION POINT 1: Early check before starting any work
	// Prevents starting any operations if shutdown is already requested
	if ctx.Err() != nil {
		return false
	}

	// Begin a transaction
	jm.logger.Debug0().LogActivity("Starting transaction for row status updates", map[string]any{
		"iteration": "start",
	})
	txStartTime := time.Now()
	tx, err := jm.db.Begin(ctx)
	if err != nil {
		jm.logger.Error(err).LogActivity("Error starting transaction", map[string]any{
			"elapsedMs": time.Since(txStartTime).Milliseconds(),
		})
		return false
	}
	jm.logger.Debug0().LogActivity("Transaction started successfully", map[string]any{
		"elapsedMs": time.Since(txStartTime).Milliseconds(),
	})
	// Critical: Ensure transaction is rolled back if not committed.
	// This defer MUST be immediately after Begin() to handle all error paths including panics.
	// Previously, missing this defer could leave transactions hanging if a panic occurred,
	// causing database connection pool exhaustion and row locks that prevent retry.
	// The tx=nil pattern after Commit() prevents unnecessary rollback attempts.
	defer func() {
		if tx != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && rollbackErr.Error() != "tx is closed" {
				jm.logger.Debug0().LogActivity("Transaction rollback attempted", map[string]any{
					"error": rollbackErr.Error(),
				})
			}
		}
	}()

	// Create a new Queries instance using the transaction
	txQueries := batchsqlc.New(tx)

	// Fetch a block of rows from the database
	jm.logger.Debug0().LogActivity("Fetching block of rows", map[string]any{
		"status": "queued",
		"limit": jm.config.BatchChunkNRows,
	})
	blockOfRows, err := txQueries.FetchBlockOfRows(ctx, batchsqlc.FetchBlockOfRowsParams{
		Status: batchsqlc.StatusEnumQueued,
		Limit:  int32(jm.config.BatchChunkNRows),
	})
	if err != nil {
		jm.logger.Error(err).LogActivity("Error fetching block of rows", nil)
		tx.Rollback(ctx)
		return false
	}

	// If no rows are found, rollback and return false
	if len(blockOfRows) == 0 {
		jm.logger.Debug0().LogActivity("No rows found in queue, rolling back transaction", nil)
		if err := tx.Rollback(ctx); err != nil {
			jm.logger.Error(err).LogActivity("Error rolling back transaction", nil)
		}
		return false
	}

	// Log which batches and how many rows per batch were fetched
	batchRowCounts := make(map[string]int)
	for _, row := range blockOfRows {
		batchRowCounts[row.Batch.String()]++
	}
	jm.logger.Info().LogActivity("Processing batch rows", map[string]any{
		"rowCount": len(blockOfRows),
		"batchRowCounts": batchRowCounts,
	})

	// BULK STATUS UPDATE OPTIMIZATION:
	// Instead of updating each row/batch individually in a loop (2N queries),
	// we collect all IDs and perform bulk updates (2 queries total).
	// This significantly improves performance and reduces database load.
	//
	// The order is important:
	// 1. First update batch statuses - this marks batches as being processed
	// 2. Then update row statuses - this marks individual rows as being processed
	// This ensures that if we fail between these operations, we can identify
	// partially processed batches.

	// Step 1: Collect unique batch IDs and row IDs
	batchIDSet := make(map[uuid.UUID]bool)
	rowIDs := make([]int64, 0, len(blockOfRows))
	
	for _, row := range blockOfRows {
		// Only collect batches that are currently 'queued' (need status update)
		if row.Status == batchsqlc.StatusEnumQueued {
			batchIDSet[row.Batch] = true
		}
		rowIDs = append(rowIDs, row.Rowid)
	}

	// Convert batch ID set to slice
	batchIDs := make([]uuid.UUID, 0, len(batchIDSet))
	for batchID := range batchIDSet {
		batchIDs = append(batchIDs, batchID)
	}

	// CANCELLATION POINT 2: Before status updates
	if ctx.Err() != nil {
		tx.Rollback(ctx)
		return false
	}

	// Step 2: Bulk update batch statuses (only for batches that need it)
	if len(batchIDs) > 0 {
		jm.logger.Debug0().LogActivity("Bulk updating batch statuses to inprog", map[string]any{
			"batchCount": len(batchIDs),
			"batchIDs": batchIDs,
		})
		
		err := txQueries.UpdateBatchesStatusBulk(ctx, batchsqlc.UpdateBatchesStatusBulkParams{
			BatchIds: batchIDs,
			Status:   batchsqlc.StatusEnumInprog,
		})
		if err != nil {
			jm.logger.Error(err).LogActivity("Error bulk updating batch statuses", map[string]any{
				"batchCount": len(batchIDs),
			})
			tx.Rollback(ctx)
			return false
		}
		
		// Log the batch status changes
		changeDetails := logharbour.ChangeInfo{
			Entity: "Batch",
			Op:     "StatusUpdated",
			Changes: []logharbour.ChangeDetail{
				{"status", batchsqlc.StatusEnumQueued, batchsqlc.StatusEnumInprog},
			},
		}
		jm.logger.LogDataChange(fmt.Sprintf("%d batch statuses updated to inprog", len(batchIDs)), changeDetails)
	}

	// Step 3: Bulk update all row statuses
	jm.logger.Debug0().LogActivity("Bulk updating batch row statuses to inprog", map[string]any{
		"rowCount": len(rowIDs),
		"rowIDs": rowIDs,
	})
	
	err = txQueries.UpdateBatchRowsStatusBulk(ctx, batchsqlc.UpdateBatchRowsStatusBulkParams{
		RowIds: rowIDs,
		Status: batchsqlc.StatusEnumInprog,
	})
	if err != nil {
		jm.logger.Error(err).LogActivity("Error bulk updating batch row statuses", map[string]any{
			"rowCount": len(rowIDs),
		})
		tx.Rollback(ctx)
		return false
	}

	// CANCELLATION POINT 3: Before committing first transaction
	// Avoids committing changes if shutdown was requested during row status updates
	// Ensures database consistency by rolling back incomplete work
	if ctx.Err() != nil {
		tx.Rollback(ctx)
		return false
	}

	jm.logger.Debug0().LogActivity("Committing transaction for status updates", map[string]any{
		"rowCount": len(blockOfRows),
	})
	err = tx.Commit(ctx)
	if err != nil {
		jm.logger.Error(err).LogActivity("Error committing transaction", nil)
		return false
	}
	// Set tx to nil to prevent rollback in defer
	tx = nil

	// Track all rows in Redis for crash recovery
	// This allows other instances to recover these rows if this instance crashes
	for _, row := range blockOfRows {
		if err := jm.TrackRowProcessing(ctx, row.Rowid); err != nil {
			jm.logger.Warn().LogActivity("Failed to track row in Redis", map[string]any{
				"rowId": row.Rowid,
				"error": err.Error(),
			})
		}
	}

	// Process the rows
	// Track which batch+app+op combinations we've already processed for errors
	processedErrorCombinations := make(map[string]bool)

	for _, row := range blockOfRows {
		// No cancellation point here. Once rows are committed as inprog,
		// we finish processing them all. If SIGKILL arrives before completion,
		// crash recovery resets the remaining rows via heartbeat expiry.

		// send queries instance, not transaction
		q := jm.queries
		jm.logger.Debug0().LogActivity("Processing row", map[string]any{
			"rowId": row.Rowid,
			"batchId": row.Batch.String(),
			"app": row.App,
			"op": row.Op,
			"line": row.Line,
		})
		_, err = jm.processRow(q, row)

		if untrackErr := jm.UntrackRowProcessing(row.Rowid); untrackErr != nil {
			jm.logger.Warn().LogActivity("Failed to untrack row from Redis", map[string]any{
				"rowId": row.Rowid,
				"error": untrackErr.Error(),
			})
		}

		if err != nil {
			jm.logger.Error(err).LogActivity("Error processing row", map[string]any{
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

	// Create a map to store unique batch IDs
	batchSet := make(map[uuid.UUID]bool)
	for _, row := range blockOfRows {
		batchSet[row.Batch] = true
	}

	// Log which batches will be checked for summarization
	batchIdsForSummary := make([]string, 0, len(batchSet))
	for batchID := range batchSet {
		batchIdsForSummary = append(batchIdsForSummary, batchID.String())
	}

	// Check for completed batches and summarize them
	// Each batch now gets its own individual transaction
	jm.logger.Info().LogActivity("Summarizing completed batches with individual transactions", map[string]any{
		"batchCount": len(batchSet),
		"batchIds": batchIdsForSummary,
	})
	if err := jm.summarizeCompletedBatches(ctx, batchSet); err != nil {
		// Check if error was due to context cancellation
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			jm.logger.Debug0().LogActivity("Batch summarization cancelled due to context", nil)
			return true // Return true because we did process rows
		}
		jm.logger.Error(err).LogActivity("Error summarizing completed batches", nil)
		return true // Return true because we did process rows
	}

	// Close and clean up initblocks
	jm.closeInitBlocks()
	
	// Return true to indicate rows were processed
	return true
}

func (jm *JobManager) processRow(txQueries batchsqlc.Querier, row batchsqlc.FetchBlockOfRowsRow) (status batchsqlc.StatusEnum, err error) {
	// Row-level recovery: MUST ALWAYS recover to isolate failures.
	// This layer handles individual row failures without affecting other rows.
	// 
	// Why we MUST recover here:
	// 1. One bad row shouldn't stop processing of other rows in the batch
	// 2. User-provided processors might panic on specific data
	// 3. We need to properly mark the row as failed in the database
	// 
	// The panic is converted to an error so handleProcessingError can update 
	// the row status appropriately. This is pure error handling, not health decisions.
	defer func() {
		if r := recover(); r != nil {
			panicDetails := map[string]any{
				"panic": fmt.Sprintf("%v", r),
				"rowId": row.Rowid,
				"batchId": row.Batch.String(),
				"app": row.App,
				"op": row.Op,
				"line": row.Line,
				"jobType": row.Op,
				"stackTrace": string(debug.Stack()),
			}
			jm.logger.Error(fmt.Errorf("panic recovered: %v", r)).LogActivity("Panic in processRow", panicDetails)
			
			// Log specific panic location for debugging
			stackStr := string(debug.Stack())
			if strings.Contains(stackStr, "DoSlowQuery") {
				jm.logger.Error(nil).LogActivity("Panic occurred in SlowQuery processor", panicDetails)
			} else if strings.Contains(stackStr, "DoBatchJob") {
				jm.logger.Error(nil).LogActivity("Panic occurred in Batch processor", panicDetails)
			} else if strings.Contains(stackStr, "getOrCreateInitBlock") {
				jm.logger.Error(nil).LogActivity("Panic occurred in getOrCreateInitBlock", panicDetails)
			}
			status = batchsqlc.StatusEnumFailed
			err = fmt.Errorf("panic during row processing: %v", r)
		}
	}()

	// Process the row based on its type (slow query or batch job)
	if row.Line == 0 {
		jm.logger.Debug0().LogActivity("Processing slow query", map[string]any{
			"rowId": row.Rowid,
			"batchId": row.Batch.String(),
			"app": row.App,
			"op": row.Op,
		})
		return jm.processSlowQuery(txQueries, row)
	} else {
		jm.logger.Debug0().LogActivity("Processing batch job", map[string]any{
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
	jm.logger.Info().LogActivity("Starting slow query processing", map[string]any{
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
	jm.logger.Debug0().LogActivity("Getting or creating initblock for slow query", map[string]any{
		"app": row.App,
		"rowId": row.Rowid,
	})
	initBlock, err := jm.getOrCreateInitBlock(string(row.App))
	if err != nil {
		jm.logger.Error(err).LogActivity("Error getting or creating initblock", map[string]any{
			"app": row.App,
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, err
	}
	jm.logger.Debug0().LogActivity("Initblock obtained successfully", map[string]any{
		"app": row.App,
		"rowId": row.Rowid,
	})

	// Process the slow query using the registered processor
	rowContext, err := NewJSONstr(string(row.Context))
	if err != nil {
		jm.logger.Error(err).LogActivity("Error parsing row context for slow query", map[string]any{
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}
	rowInput, err := NewJSONstr(string(row.Input))
	if err != nil {
		jm.logger.Error(err).LogActivity("Error parsing row input for slow query", map[string]any{
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}
	
	jm.logger.Debug0().LogActivity("Executing slow query processor", map[string]any{
		"rowId": row.Rowid,
		"processor": fmt.Sprintf("%T", processor),
	})
	startTime := time.Now()
	status, result, messages, outputFiles, err := processor.DoSlowQuery(initBlock, rowContext, rowInput)
	elapsedTime := time.Since(startTime)
	jm.logger.Debug0().LogActivity("Slow query processor execution completed", map[string]any{
		"rowId": row.Rowid,
		"elapsedMs": elapsedTime.Milliseconds(),
		"status": status,
		"error": err != nil,
	})
	if err != nil {
		jm.logger.Error(err).LogActivity("Slow query processor failed", map[string]any{
			"rowId": row.Rowid,
			"app": row.App,
			"op": row.Op,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}
	
	jm.logger.Info().LogActivity("Slow query processor completed", map[string]any{
		"rowId": row.Rowid,
		"status": status,
		"hasMessages": len(messages) > 0,
		"hasOutputFiles": len(outputFiles) > 0,
	})

	// Update the corresponding batchrows and batches records with the results
	jm.logger.Debug0().LogActivity("Updating slow query result", map[string]any{
		"rowId": row.Rowid,
		"status": status,
	})
	if err := updateSlowQueryResult(txQueries, row, status, result, messages, outputFiles); err != nil {
		jm.logger.Error(err).LogActivity("Error updating slow query result", map[string]any{
			"rowId": row.Rowid,
			"app": row.App,
			"op": row.Op,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error updating slow query result for app %s and op %s: %v", row.App, row.Op, err)
	}
	
	// Log the row and batch status change
	rowChangeDetails := logharbour.ChangeInfo{
		Entity: "BatchRow",
		Op:     "Update",
		Changes: []logharbour.ChangeDetail{
			{"status", row.Status, status},
		},
	}
	jm.logger.LogDataChange("Slow query row updated", rowChangeDetails)
	jm.logger.Debug0().LogActivity("Slow query status updated", map[string]any{
		"rowId": row.Rowid,
		"batchId": row.Batch.String(),
		"oldStatus": row.Status,
		"newStatus": status,
	})

	jm.logger.Info().LogActivity("Slow query completed successfully", map[string]any{
		"rowId": row.Rowid,
		"batchId": row.Batch.String(),
		"status": status,
	})
	
	// Log warning if job took unusually long
	if elapsedTime > 30*time.Second {
		jm.logger.Warn().LogActivity("Slow query took unusually long", map[string]any{
			"rowId": row.Rowid,
			"batchId": row.Batch.String(),
			"elapsedSeconds": elapsedTime.Seconds(),
			"app": row.App,
			"op": row.Op,
		})
	}
	
	return status, nil
}

// processBatchJob processes a single batch job. It retrieves the registered BatchProcessor for the
// given app and op, fetches the associated InitBlock, and invokes the processor's DoBatchJob method.
// It then calls updateBatchJobResult to update the corresponding batchrows record with the processing results.
// If the processor is not found or the processing fails, an error is returned.
func (jm *JobManager) processBatchJob(txQueries batchsqlc.Querier, row batchsqlc.FetchBlockOfRowsRow) (batchsqlc.StatusEnum, error) {
	jm.logger.Info().LogActivity("Starting batch job processing", map[string]any{
		"rowId": row.Rowid,
		"batchId": row.Batch.String(),
		"app": row.App,
		"op": row.Op,
		"line": row.Line,
	})
	// Retrieve the BatchProcessor for the app and op
	p, exists := jm.batchprocessorfuncs[string(row.App)+row.Op]
	if !exists {
		jm.logger.Warn().LogActivity("Batch processor not found", map[string]any{
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
		jm.logger.Error(err).LogActivity("Error getting or creating initblock", map[string]any{
			"app": row.App,
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, err
	}

	// Process the batch job using the registered processor
	rowContext, err := NewJSONstr(string(row.Context))
	if err != nil {
		jm.logger.Error(err).LogActivity("Error parsing row context for batch job", map[string]any{
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing batch job for app %s and op %s: %v", row.App, row.Op, err)
	}
	rowInput, err := NewJSONstr(string(row.Input))
	if err != nil {
		jm.logger.Error(err).LogActivity("Error parsing row input for batch job", map[string]any{
			"rowId": row.Rowid,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing batch job for app %s and op %s: %v", row.App, row.Op, err)
	}
	
	jm.logger.Debug0().LogActivity("Executing batch processor", map[string]any{
		"rowId": row.Rowid,
		"processor": fmt.Sprintf("%T", processor),
		"line": row.Line,
	})
	startTime := time.Now()
	status, result, messages, blobRows, err := processor.DoBatchJob(initBlock, rowContext, int(row.Line), rowInput)
	elapsedTime := time.Since(startTime)
	jm.logger.Debug0().LogActivity("Batch processor execution completed", map[string]any{
		"rowId": row.Rowid,
		"line": row.Line,
		"elapsedMs": elapsedTime.Milliseconds(),
		"status": status,
		"error": err != nil,
	})
	if err != nil {
		jm.logger.Error(err).LogActivity("Batch processor failed", map[string]any{
			"rowId": row.Rowid,
			"app": row.App,
			"op": row.Op,
			"line": row.Line,
		})
	}
	
	jm.logger.Info().LogActivity("Batch processor completed", map[string]any{
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
	jm.logger.Debug0().LogActivity("Updating batch job result", map[string]any{
		"rowId": row.Rowid,
		"status": status,
	})
	if err := jm.updateBatchJobResult(txQueries, row, status, result, messages, blobRows); err != nil {
		jm.logger.Error(err).LogActivity("Error updating batch job result", map[string]any{
			"rowId": row.Rowid,
			"app": row.App,
			"op": row.Op,
		})
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error updating batch job result for app %s and op %s: %v", row.App, row.Op, err)
	}
	
	// Log the row status change
	rowChangeDetails := logharbour.ChangeInfo{
		Entity: "BatchRow",
		Op:     "Update",
		Changes: []logharbour.ChangeDetail{
			{"status", row.Status, status},
		},
	}
	jm.logger.LogDataChange("Batch row updated", rowChangeDetails)
	jm.logger.Debug0().LogActivity("Batch row status updated", map[string]any{
		"rowId": row.Rowid,
		"batchId": row.Batch.String(),
		"line": row.Line,
		"oldStatus": row.Status,
		"newStatus": status,
	})

	jm.logger.Info().LogActivity("Batch job completed successfully", map[string]any{
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

func (jm *JobManager) summarizeCompletedBatches(ctx context.Context, batchSet map[uuid.UUID]bool) error {
	jm.logger.Debug0().LogActivity("Summarizing completed batches", map[string]any{
		"batchCount": len(batchSet),
	})

	const maxRetries = 5
	const retryDelayMs = 50

	for batchID := range batchSet {
		var lastErr error

		for attempt := 1; attempt <= maxRetries; attempt++ {
			// CANCELLATION POINT: Check before each attempt
			if ctx.Err() != nil {
				jm.logger.Debug0().LogActivity("Context cancelled, stopping batch summarization", map[string]any{
					"remainingBatches": "partial",
				})
				return ctx.Err()
			}

			// Start fresh transaction for each attempt (new snapshot)
			tx, err := jm.db.Begin(ctx)
			if err != nil {
				jm.logger.Error(err).LogActivity("Failed to start transaction for batch", map[string]any{
					"batchId": batchID.String(),
					"attempt": attempt,
				})
				lastErr = err
				break // Cannot retry if cannot start transaction
			}

			txQueries := batchsqlc.New(tx)

			jm.logger.Info().LogActivity("Attempting batch summarization in individual transaction", map[string]any{
				"batchId": batchID.String(),
				"attempt": attempt,
			})

			err = jm.summarizeBatch(txQueries, batchID)

			if err == nil {
				// Success - commit and move to next batch
				commitErr := tx.Commit(ctx)
				if commitErr != nil {
					jm.logger.Error(commitErr).LogActivity("Failed to commit batch summary", map[string]any{
						"batchId": batchID.String(),
					})
					lastErr = commitErr
					break // Do not retry on commit failure
				}

				jm.logger.Info().LogActivity("Batch summarized successfully in individual transaction", map[string]any{
					"batchId": batchID.String(),
					"attempt": attempt,
				})
				lastErr = nil
				break // Success - done with this batch

			} else if errors.Is(err, ErrBatchLockNotAcquired) {
				// Lock held by another worker - rollback and retry
				tx.Rollback(ctx)
				lastErr = err

				if attempt < maxRetries {
					jm.logger.Info().LogActivity("Batch lock held by another worker, will retry", map[string]any{
						"batchId":     batchID.String(),
						"attempt":     attempt,
						"nextAttempt": attempt + 1,
						"delayMs":     retryDelayMs,
					})

					time.Sleep(time.Duration(retryDelayMs) * time.Millisecond)
					continue
				}

				jm.logger.Warn().LogActivity("Batch lock held by another worker after max retries", map[string]any{
					"batchId":       batchID.String(),
					"maxRetries":    maxRetries,
					"retryDelayMs":  retryDelayMs,
					"totalWindowMs": maxRetries * retryDelayMs,
				})
				break

			} else if errors.Is(err, ErrBatchHasPendingRows) {
				// Pending rows detected - rollback and potentially retry
				tx.Rollback(ctx)
				lastErr = err

				if attempt < maxRetries {
					jm.logger.Info().LogActivity("Batch has pending rows, will retry with fresh transaction", map[string]any{
						"batchId": batchID.String(),
						"attempt": attempt,
						"nextAttempt": attempt + 1,
						"delayMs": retryDelayMs,
					})

					// Wait before retry to allow in-flight commits to complete
					time.Sleep(time.Duration(retryDelayMs) * time.Millisecond)
					continue // Retry with new transaction (new snapshot)
				} else {
					// Max retries reached - likely real pending work, not stale snapshot
					jm.logger.Info().LogActivity("Batch has pending rows after max retries, skipping", map[string]any{
						"batchId": batchID.String(),
						"maxRetries": maxRetries,
					})
					break // Give up on this batch for now
				}

			} else {
				// Other error - rollback and skip this batch
				tx.Rollback(ctx)
				jm.logger.Error(err).LogActivity("Error summarizing batch", map[string]any{
					"batchId": batchID.String(),
					"attempt": attempt,
				})
				lastErr = err
				break // Do not retry on unexpected errors
			}
		}

		// Log final outcome for this batch
		if lastErr != nil && !errors.Is(lastErr, ErrBatchHasPendingRows) && !errors.Is(lastErr, ErrBatchLockNotAcquired) {
			jm.logger.Error(lastErr).LogActivity("Failed to summarize batch after retries", map[string]any{
				"batchId": batchID.String(),
			})
		}
	}

	return nil
}

func (jm *JobManager) closeInitBlocks() {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	
	for app, initBlock := range jm.initblocks {
		if initBlock != nil {
			if closer, ok := initBlock.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					jm.logger.Error(err).LogActivity("Error closing initblock", map[string]any{
						"app": app,
					})
				}
			}
		}
	}
	jm.initblocks = make(map[string]InitBlock)
}

func (jm *JobManager) getRandomSleepDuration() time.Duration {
	// Use the configured polling interval with some randomization (+/- 33%)
	baseInterval := jm.config.PollingIntervalSec
	if baseInterval == 0 {
		baseInterval = ALYA_POLLING_INTERVAL_SEC
	}
	
	// Add randomization: between 2/3 and 4/3 of the base interval
	minInterval := baseInterval * 2 / 3
	maxInterval := baseInterval * 4 / 3
	randomRange := maxInterval - minInterval + 1
	
	return time.Duration(rand.Intn(randomRange)+minInterval) * time.Second
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
	err = jm.queries.UpdateBatchRowsBatchJob(context.Background(), batchsqlc.UpdateBatchRowsBatchJobParams{
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
	err = jm.queries.UpdateBatchRowsByBatchAppOp(context.Background(), batchsqlc.UpdateBatchRowsByBatchAppOpParams{
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
	err := jm.queries.UpdateBatchStatus(context.Background(), batchsqlc.UpdateBatchStatusParams{
		ID:     batchID,
		Status: batchsqlc.StatusEnumFailed,
		Doneat: pgtype.Timestamp{Time: time.Now(), Valid: true},
	})

	if err == nil {
		// Also update Redis cache if applicable
		if jm.redisClient != nil {
			cacheErr := updateStatusInRedis(jm.redisClient, batchID, batchsqlc.StatusEnumFailed,
				jm.config.BatchStatusCacheDurSec)
			if cacheErr != nil {
				jm.logger.Warn().LogActivity("Failed to update Redis cache for batch", map[string]any{
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
	jm.logger.Info().LogActivity("Starting error handling for job", map[string]any{
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
				jm.logger.Error(failErr).LogActivity("Error marking all rows with same batch+app+op as failed", map[string]any{
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
				jm.logger.Error(failErr).LogActivity("Error marking all rows with same batch+app as failed", map[string]any{
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
				jm.logger.Error(rowErr).LogActivity("Error updating row status to failed", map[string]any{
					"rowID":   row.Rowid,
					"batchID": batchID.String(),
					"errCode": errCode,
				})
			}
		}

		// Mark the current row as failed
		if failErr := jm.updateRowStatusToFailed(row.Rowid, errCode, msgID, err.Error()); failErr != nil {
			jm.logger.Error(failErr).LogActivity("Error updating row status to failed", map[string]any{
				"rowID":   row.Rowid,
				"batchID": batchID.String(),
				"errCode": errCode,
			})
		}

		// Update the batch status to failed
		if failErr := jm.updateBatchStatusToFailed(batchID); failErr != nil {
			jm.logger.Error(failErr).LogActivity("Error updating batch status to failed", map[string]any{
				"batchID": batchID.String(),
				"errCode": errCode,
			})
		} else {
			jm.logger.Info().LogActivity("Batch marked as failed due to configuration error", map[string]any{
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
			jm.logger.Error(failErr).LogActivity("Error updating row status to failed", map[string]any{
				"rowID":   row.Rowid,
				"batchID": batchID.String(),
				"errCode": errCode,
			})
		} else {
			jm.logger.Info().LogActivity("Row marked as failed due to processing error", map[string]any{
				"rowID":   row.Rowid,
				"batchID": batchID.String(),
				"app":     app,
				"op":      op,
				"error":   err.Error(),
			})
		}

		// Quick fix for slow queries: If this is a slow query (line=0), update the batch status immediately
		// since slow queries have only one row per batch
		if row.Line == 0 {
			jm.logger.Info().LogActivity("Updating batch status for failed slow query", map[string]any{
				"batchID": batchID.String(),
				"rowID":   row.Rowid,
			})
			
			// Update the batch status to failed
			// TODO: The current processSlowQuery implementation returns immediately on error without
			// preserving any outputFiles that the processor might have generated. We should consider
			// modifying processSlowQuery to capture and store partial output files even when the
			// processor returns an error, as these might be useful for debugging or recovery.
			err := jm.queries.UpdateBatchResult(context.Background(), batchsqlc.UpdateBatchResultParams{
				ID:          batchID,
				Status:      batchsqlc.StatusEnumFailed,
				Doneat:      pgtype.Timestamp{Time: time.Now(), Valid: true},
				Outputfiles: nil, // No output files for failed slow query
			})
			if err != nil {
				jm.logger.Error(err).LogActivity("Error updating batch status for failed slow query", map[string]any{
					"batchID": batchID.String(),
				})
			}
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
	err = jm.queries.UpdateBatchRowsByBatchApp(context.Background(), batchsqlc.UpdateBatchRowsByBatchAppParams{
		Status:   batchsqlc.StatusEnumFailed,
		Doneat:   pgtype.Timestamp{Time: time.Now(), Valid: true},
		Messages: messagesJSON,
		ID:       batchID,
		App:      app,
	})

	return err
}
