package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/remiges-tech/alya/batch/objstore"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

const ALYA_BATCHCHUNK_NROWS = 10

// Assuming global variables are defined elsewhere
// make all the maps sync maps to make them thread safe
var (
	mu     sync.Mutex // Ensures thread-safe access to the initfuncs map
	doneBy pgtype.Text
)

type JobManager struct {
	Db                      *pgxpool.Pool
	Queries                 batchsqlc.Querier
	RedisClient             *redis.Client
	ObjStore                objstore.ObjectStore
	initblocks              map[string]InitBlock
	initfuncs               map[string]Initializer
	slowqueryprocessorfuncs map[string]SlowQueryProcessor
	batchprocessorfuncs     map[string]BatchProcessor
}

func NewJobManager(db *pgxpool.Pool, redisClient *redis.Client, minioClient *minio.Client) *JobManager {
	return &JobManager{
		Db:                      db,
		Queries:                 batchsqlc.New(db),
		RedisClient:             redisClient,
		ObjStore:                objstore.NewMinioObjectStore(minioClient),
		initblocks:              make(map[string]InitBlock),
		initfuncs:               make(map[string]Initializer),
		slowqueryprocessorfuncs: make(map[string]SlowQueryProcessor),
		batchprocessorfuncs:     make(map[string]BatchProcessor),
	}
}

// RegisterInitializer registers an initializer for a specific application.
// This is where applications register their initial logic with Alya.
func (jm *JobManager) RegisterInitializer(app string, initializer Initializer) error {
	mu.Lock()
	defer mu.Unlock()

	// Check if an initializer for this app already exists to prevent accidental overwrites
	if _, exists := jm.initfuncs[app]; exists {
		return fmt.Errorf("initializer for app %s already registered", app)
	}

	// Register the initializer for the app
	jm.initfuncs[app] = initializer
	return nil
}

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
			Limit:  ALYA_BATCHCHUNK_NROWS,
		})
		if err != nil {
			log.Println("Error fetching block of rows:", err)
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

		batchSet := make(map[uuid.UUID]bool)

		// Process each row in the block
		for _, row := range blockOfRows {
			_, err := jm.processRow(txQueries, row)
			if err != nil {
				log.Println("Error processing row:", err)
				continue
			}

			// Add the batch ID to the batchSet if it's not a slow query
			if row.Line != 0 {
				batchSet[row.Batch] = true
			}
		}

		// Check for completed batches and summarize them
		if err := jm.summarizeCompletedBatches(txQueries, batchSet); err != nil {
			log.Println("Error summarizing completed batches:", err)
		}

		tx.Commit(ctx)

		// Close and clean up initblocks
		jm.closeInitBlocks()
	}
}

func (jm *JobManager) processRow(txQueries *batchsqlc.Queries, row batchsqlc.FetchBlockOfRowsRow) (batchsqlc.StatusEnum, error) {
	fmt.Printf("jobmanager inside processrow\n")

	// Process the row based on its type (slow query or batch job)
	if row.Line == 0 {
		return jm.processSlowQuery(txQueries, row)
	} else {
		return jm.processBatchJob(txQueries, row)
	}
}

func (jm *JobManager) processSlowQuery(txQueries *batchsqlc.Queries, row batchsqlc.FetchBlockOfRowsRow) (batchsqlc.StatusEnum, error) {
	log.Printf("processing slow query for app %s and op %s", row.App, row.Op)
	// Retrieve the SlowQueryProcessor for the app and op
	processor, exists := jm.slowqueryprocessorfuncs[string(row.App)+row.Op]
	if !exists {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("no SlowQueryProcessor registered for app %s and op %s", row.App, row.Op)
	}

	// Get or create the initblock for the app
	initBlock, err := jm.getOrCreateInitBlock(string(row.App))
	if err != nil {
		log.Printf("error getting or creating initblock for app %s: %v", string(row.App), err)
		return batchsqlc.StatusEnumFailed, err
	}

	// Process the slow query using the registered processor
	status, result, messages, outputFiles, err := processor.DoSlowQuery(initBlock, JSONstr(string(row.Context)), JSONstr(string(row.Input)))
	if err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}

	// Update the corresponding batchrows and batches records with the results
	if err := updateSlowQueryResult(txQueries, row, status, result, messages, outputFiles); err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error updating slow query result for app %s and op %s: %v", row.App, row.Op, err)
	}

	return status, nil
}

func (jm *JobManager) processBatchJob(txQueries *batchsqlc.Queries, row batchsqlc.FetchBlockOfRowsRow) (batchsqlc.StatusEnum, error) {
	// Retrieve the BatchProcessor for the app and op
	processor, exists := jm.batchprocessorfuncs[string(row.App)+row.Op]
	if !exists {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("no BatchProcessor registered for app %s and op %s", row.App, row.Op)
	}

	// Get or create the initblock for the app
	initBlock, err := jm.getOrCreateInitBlock(string(row.App))
	if err != nil {
		log.Printf("error getting or creating initblock for app %s: %v", string(row.App), err)
		return batchsqlc.StatusEnumFailed, err
	}

	// Process the batch job using the registered processor
	status, result, messages, blobRows, err := processor.DoBatchJob(initBlock, JSONstr(string(row.Context)), int(row.Line), JSONstr(string(row.Input)))
	if err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing batch job for app %s and op %s: %v", row.App, row.Op, err)
	}

	// Update the corresponding batchrows record with the results
	if err := updateBatchJobResult(txQueries, row, status, result, messages, blobRows); err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error updating batch job result for app %s and op %s: %v", row.App, row.Op, err)
	}

	return status, nil
}

func updateSlowQueryResult(txQueries *batchsqlc.Queries, row batchsqlc.FetchBlockOfRowsRow, status batchsqlc.StatusEnum, result JSONstr, messages []wscutils.ErrorMessage, outputFiles map[string]string) error {
	// Marshal messages to JSON
	var messagesJSON []byte
	if len(messages) > 0 {
		var err error
		messagesJSON, err = json.Marshal(messages)
		if err != nil {
			return fmt.Errorf("failed to marshal messages to JSON: %v", err)
		}
	}

	// Update the batchrows record with the results
	err := txQueries.UpdateBatchRowsSlowQuery(context.Background(), batchsqlc.UpdateBatchRowsSlowQueryParams{
		Rowid:    int32(row.Rowid),
		Status:   batchsqlc.StatusEnum(status),
		Doneat:   pgtype.Timestamp{Time: time.Now()},
		Res:      []byte(result),
		Messages: messagesJSON,
		Doneby:   doneBy,
	})
	if err != nil {
		return err
	}

	return nil
}

func updateBatchJobResult(txQueries *batchsqlc.Queries, row batchsqlc.FetchBlockOfRowsRow, status batchsqlc.StatusEnum, result JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string) error {
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
		Rowid:    int32(row.Rowid),
		Status:   batchsqlc.StatusEnum(status),
		Doneat:   pgtype.Timestamp{Time: time.Now()},
		Res:      []byte(result),
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
		if err := initBlock.Close(); err != nil {
			log.Println("Error closing initblock for app:", app, err)
		}
	}
	jm.initblocks = make(map[string]InitBlock)
}

func getRandomSleepDuration() time.Duration {
	// Generate a random sleep duration between 30 and 60 seconds
	return time.Duration(rand.Intn(31)+30) * time.Second
}
