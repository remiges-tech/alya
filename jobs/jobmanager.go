package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	// Lock the mutex to ensure thread-safe access to the initblocks map
	mu.Lock()
	defer mu.Unlock()

	// Check if an InitBlock already exists for the app
	if initBlock, exists := jm.initblocks[app]; exists {
		return initBlock, nil
	}
	// Check if an Initializer is registered for the app
	initializer, exists := jm.initfuncs[app]
	if !exists {
		log.Printf("no initializer registered for app %s", app)
		return nil, fmt.Errorf("no initializer registered for app %s", app)
	}

	// Create a new InitBlock using the registered Initializer
	initBlock, err := initializer.Init(app)
	if err != nil {
		return nil, fmt.Errorf("error initializing InitBlock for app %s: %v", app, err)
	}

	// Cache the InitBlock for future use
	jm.initblocks[app] = initBlock

	return initBlock, nil
}

// Run is the main loop of the JobManager. It continuously fetches a block of rows from the database,
// processes each row either as a slow query or a batch job. After processing a block, it checks for
// completed batches and summarizes them. Fetching, processing and updating happens in the same transaction.
// This method should be called in a separate goroutine. It is thread safe -- updates to database and Redis
// are executed atomically (check updateStatusInRedis()).
func (jm *JobManager) Run() {
	for {
		ctx := context.Background()

		// Begin a transaction
		tx, err := jm.Db.Begin(ctx)
		if err != nil {
			log.Println("Error starting transaction:", err)
			time.Sleep(getRandomSleepDuration())
			continue
		}

		// Create a new Queries instance using the transaction
		txQueries := batchsqlc.New(tx)

		// Fetch a block of rows from the database
		blockOfRows, err := txQueries.FetchBlockOfRows(ctx, batchsqlc.FetchBlockOfRowsParams{
			Status: batchsqlc.StatusEnumQueued,
			Limit:  int32(jm.Config.BatchChunkNRows),
		})
		if err != nil {
			log.Println("Error fetching block of rows:", err)
			tx.Rollback(ctx)
			time.Sleep(getRandomSleepDuration())
			continue
		}

		// If no rows are found, sleep and continue
		if len(blockOfRows) == 0 {
			log.Println("No rows found, sleeping...")
			tx.Rollback(ctx)
			time.Sleep(getRandomSleepDuration())
			continue
		}

		// Process each row in the block
		for _, row := range blockOfRows {
			// Update the status of the batch row to "inprog"
			err := txQueries.UpdateBatchRowStatus(ctx, batchsqlc.UpdateBatchRowStatusParams{
				Rowid:  row.Rowid,
				Status: batchsqlc.StatusEnumInprog,
			})
			if err != nil {
				log.Println("Error updating batch row status:", err)
				tx.Rollback(ctx)
				time.Sleep(getRandomSleepDuration())
				continue
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
				err := txQueries.UpdateBatchStatus(ctx, batchsqlc.UpdateBatchStatusParams{
					ID:     row.Batch,
					Status: batchsqlc.StatusEnumInprog,
				})
				if err != nil {
					log.Println("Error updating batch status:", err)
					tx.Rollback(ctx)
					time.Sleep(getRandomSleepDuration())
					continue
				}
			}

		}

		// let us commit the transaction
		err = tx.Commit(ctx)
		if err != nil {
			log.Println("Error committing transaction:", err)
			time.Sleep(getRandomSleepDuration())
			continue
		}

		// Process the rows
		for _, row := range blockOfRows {
			// send queries instance, not transaction
			q := jm.Queries
			_, err = jm.processRow(q, row)
			if err != nil {
				log.Println("Error processing row:", err)
				time.Sleep(getRandomSleepDuration())
				continue
			}
		}

		// create a new transaction for the summarizeCompletedBatches
		tx, err = jm.Db.Begin(ctx)
		if err != nil {
			log.Println("Error starting transaction:", err)
			time.Sleep(getRandomSleepDuration())
			continue
		}

		txQueries = batchsqlc.New(tx)

		// Create a map to store unique batch IDs
		batchSet := make(map[uuid.UUID]bool)
		for _, row := range blockOfRows {
			batchSet[row.Batch] = true
		}

		// Check for completed batches and summarize them
		if err := jm.summarizeCompletedBatches(txQueries, batchSet); err != nil {
			log.Println("Error summarizing completed batches:", err)
		}

		// Commit the transaction after processing the entire block
		err = tx.Commit(ctx)
		if err != nil {
			log.Println("Error committing transaction:", err)
			time.Sleep(getRandomSleepDuration())
			continue
		}

		// Close and clean up initblocks
		jm.closeInitBlocks()
	}
}

func (jm *JobManager) processRow(txQueries batchsqlc.Querier, row batchsqlc.FetchBlockOfRowsRow) (batchsqlc.StatusEnum, error) {
	fmt.Printf("jobmanager inside processrow\n")

	// Process the row based on its type (slow query or batch job)
	if row.Line == 0 {
		return jm.processSlowQuery(txQueries, row)
	} else {
		return jm.processBatchJob(txQueries, row)
	}
}

// processSlowQuery processes a single slow query job. It retrieves the registered SlowQueryProcessor
// for the given app and op, fetches the associated InitBlock, and invokes the processor's DoSlowQuery
// method. It then calls updateSlowQueryResult to update the corresponding batchrows and batches records
// with the processing results. If the processor is not found or the processing fails, an error is returned.

func (jm *JobManager) processSlowQuery(txQueries batchsqlc.Querier, row batchsqlc.FetchBlockOfRowsRow) (batchsqlc.StatusEnum, error) {
	log.Printf("processing slow query for app %s and op %s", row.App, row.Op)
	// Retrieve the SlowQueryProcessor for the app and op
	p, exists := jm.slowqueryprocessorfuncs[string(row.App)+row.Op]
	if !exists {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("no SlowQueryProcessor registered for app %s and op %s", row.App, row.Op)
	}

	// Assert that the processor is of the correct type
	processor, ok := p.(SlowQueryProcessor)
	if !ok {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("invalid SlowQueryProcessor type for app %s and op %s", row.App, row.Op)
	}

	// Get or create the initblock for the app
	initBlock, err := jm.getOrCreateInitBlock(string(row.App))
	if err != nil {
		log.Printf("error getting or creating initblock for app %s: %v", string(row.App), err)
		return batchsqlc.StatusEnumFailed, err
	}

	// Process the slow query using the registered processor
	rowContext, err := NewJSONstr(string(row.Context))
	if err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}
	rowInput, err := NewJSONstr(string(row.Input))
	if err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}
	status, result, messages, outputFiles, err := processor.DoSlowQuery(initBlock, rowContext, rowInput)
	if err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}

	// Update the corresponding batchrows and batches records with the results
	if err := updateSlowQueryResult(txQueries, row, status, result, messages, outputFiles); err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error updating slow query result for app %s and op %s: %v", row.App, row.Op, err)
	}

	return status, nil
}

// processBatchJob processes a single batch job. It retrieves the registered BatchProcessor for the
// given app and op, fetches the associated InitBlock, and invokes the processor's DoBatchJob method.
// It then calls updateBatchJobResult to update the corresponding batchrows record with the processing results.
// If the processor is not found or the processing fails, an error is returned.
func (jm *JobManager) processBatchJob(txQueries batchsqlc.Querier, row batchsqlc.FetchBlockOfRowsRow) (batchsqlc.StatusEnum, error) {
	// Retrieve the BatchProcessor for the app and op
	p, exists := jm.batchprocessorfuncs[string(row.App)+row.Op]
	if !exists {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("no BatchProcessor registered for app %s and op %s", row.App, row.Op)
	}

	// Assert that the processor is of the correct type
	processor, ok := p.(BatchProcessor)
	if !ok {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("invalid BatchProcessor type for app %s and op %s", row.App, row.Op)
	}

	// Get or create the initblock for the app
	initBlock, err := jm.getOrCreateInitBlock(string(row.App))
	if err != nil {
		log.Printf("error getting or creating initblock for app %s: %v", string(row.App), err)
		return batchsqlc.StatusEnumFailed, err
	}

	// Process the batch job using the registered processor
	rowContext, err := NewJSONstr(string(row.Context))
	if err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing batch job for app %s and op %s: %v", row.App, row.Op, err)
	}
	rowInput, err := NewJSONstr(string(row.Input))
	if err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing batch job for app %s and op %s: %v", row.App, row.Op, err)
	}
	status, result, messages, blobRows, err := processor.DoBatchJob(initBlock, rowContext, int(row.Line), rowInput)
	// TODO: check if it should return the error and what its effects are
	// if err != nil {
	// return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing batch job for app %s and op %s: %v", row.App, row.Op, err)
	// }

	// Update the corresponding batchrows record with the results
	if err := jm.updateBatchJobResult(txQueries, row, status, result, messages, blobRows); err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error updating batch job result for app %s and op %s: %v", row.App, row.Op, err)
	}

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
		log.Fatalf("Failed to marshal outputFiles to JSON: %v", err)
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
	fmt.Printf("jobmanager inside summarizecompletedbatches\n")
	for batchID := range batchSet {
		if err := jm.summarizeBatch(q, batchID); err != nil {
			log.Println("Error summarizing batch:", batchID, err)
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
					log.Println("Error closing initblock for app:", app, err)
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
