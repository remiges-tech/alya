package batch

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

type BatchJob_t struct {
	App     string
	Op      string
	Batch   string
	RowID   int
	Context JSONstr
	Line    int
	Input   JSONstr
}

type InitBlock interface {
	Close() error
}

type Initializer interface {
	Init(app string) (InitBlock, error)
}

type SlowQueryProcessor interface {
	DoSlowQuery(InitBlock any, context JSONstr, input JSONstr) (status BatchStatus_t, result JSONstr, messages []wscutils.ErrorMessage, outputFiles map[string]string, err error)
}

type BatchProcessor interface {
	DoBatchJob(InitBlock any, context JSONstr, line int, input JSONstr) (status BatchStatus_t, result JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string, err error)
}

// Assuming global variables are defined elsewhere
// make all the maps sync maps to make them thread safe
var (
	initblocks              map[string]InitBlock
	initfuncs               map[string]Initializer
	slowqueryprocessorfuncs map[string]SlowQueryProcessor
	batchprocessorfuncs     map[string]BatchProcessor
	mu                      sync.Mutex // Ensures thread-safe access to the initfuncs map
	doneBy                  pgtype.Text
)

// RegisterInitializer registers an initializer for a specific application.
// This is where applications register their initial logic with Alya.
func RegisterInitializer(app string, initializer Initializer) error {
	mu.Lock()
	defer mu.Unlock()

	// Check if an initializer for this app already exists to prevent accidental overwrites
	if _, exists := initfuncs[app]; exists {
		return fmt.Errorf("initializer for app %s already registered", app)
	}

	// Register the initializer for the app
	initfuncs[app] = initializer
	return nil
}

func getOrCreateInitBlock(app string) (InitBlock, error) {
	// Lock the mutex to ensure thread-safe access to the initblocks map
	mu.Lock()
	defer mu.Unlock()

	// Check if an InitBlock already exists for the app
	if initBlock, exists := initblocks[app]; exists {
		return initBlock, nil
	}

	// Check if an Initializer is registered for the app
	initializer, exists := initfuncs[app]
	if !exists {
		return nil, fmt.Errorf("no initializer registered for app %s", app)
	}

	// Create a new InitBlock using the registered Initializer
	initBlock, err := initializer.Init(app)
	if err != nil {
		return nil, fmt.Errorf("error initializing InitBlock for app %s: %v", app, err)
	}

	// Cache the InitBlock for future use
	initblocks[app] = initBlock

	return initBlock, nil
}

// JobManager is responsible for pulling queued jobs, processing them, and updating records accordingly
func JobManager(db *sql.DB) {
	for {
		// Fetch a block of rows from the database
		blockOfRows, err := fetchBlockOfRows(db)
		if err != nil {
			log.Println("Error fetching block of rows:", err)
			time.Sleep(getRandomSleepDuration())
			continue
		}

		// If no rows are found, sleep and continue
		if len(blockOfRows) == 0 {
			log.Println("No rows found, sleeping...")
			time.Sleep(getRandomSleepDuration())
			continue
		}

		// Process each row in the block
		for _, row := range blockOfRows {
			if err := processRow(db, row); err != nil {
				log.Println("Error processing row:", err)
			}
		}

		// Check for completed batches and summarize them
		if err := summarizeCompletedBatches(db); err != nil {
			log.Println("Error summarizing completed batches:", err)
		}

		// Close and clean up initblocks
		closeInitBlocks()
	}
}

func fetchBlockOfRows(db *batchsqlc.Queries) ([]BatchJob_t, error) {
	// Begin a transaction
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Fetch a block of rows with status "queued"
	rows, err := tx.FetchBlockOfRows(ctx, batchsqlc.FetchBlockOfRowsParams{
		Status: batchsqlc.StatusEnumQueued,
		Limit:  100,
	})
	if err != nil {
		return nil, err
	}

	var blockOfRows []BatchJob_t
	for _, row := range rows {
		job := BatchJob_t{
			App:   string(row.App),
			Op:    row.Op,
			Batch: row.Batch.String(),
			RowID: int(row.Rowid),
			Line:  int(row.Line),
			Input: row.Input,
		}
		blockOfRows = append(blockOfRows, job)
	}

	// Update the fetched rows' status to "inprog"
	rowIDs := make([]int32, len(blockOfRows))
	for i, job := range blockOfRows {
		rowIDs[i] = int32(job.RowID)
	}

	err = tx.UpdateBatchRowsStatus(ctx, batchsqlc.UpdateBatchRowsStatusParams{
		Status: batchsqlc.StatusEnumInprog,
		Rowids: rowIDs,
	})
	if err != nil {
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return blockOfRows, nil
}

func processRow(db *sql.DB, row BatchJob_t) error {
	// Check if the row is valid for processing
	if row.Line == -1 {
		return nil
	}

	// Get or create the initblock for the app
	initBlock, err := getOrCreateInitBlock(row.App)
	if err != nil {
		return err
	}

	// Process the row based on its type (slow query or batch job)
	if row.Line == 0 {
		if err := processSlowQuery(db, row, initBlock); err != nil {
			return err
		}
	} else {
		if err := processBatchJob(db, row, initBlock); err != nil {
			return err
		}
	}

	return nil
}

func summarizeCompletedBatches(db *sql.DB) error {
	// Retrieve completed batches
	completedBatches, err := getCompletedBatches(db)
	if err != nil {
		return err
	}

	// Summarize each completed batch
	for _, batch := range completedBatches {
		if err := summarizeBatch(db, batch); err != nil {
			log.Println("Error summarizing batch:", batch, err)
		}
	}

	return nil
}

func closeInitBlocks() {
	for app, initBlock := range initblocks {
		if err := initBlock.Close(); err != nil {
			log.Println("Error closing initblock for app:", app, err)
		}
	}
	initblocks = make(map[string]InitBlock)
}

func getRandomSleepDuration() time.Duration {
	// Generate a random sleep duration between 30 and 60 seconds
	return time.Duration(rand.Intn(31)+30) * time.Second
}

func cleanup() {
	// Cleanup and reset global variables as necessary
}

// fetchJobs function queries the database for queued jobs,
// lock them for processing (e.g., by setting their status to inprog),
// and return them for processing.
func fetchJobs(tx *sql.Tx) []BatchJob_t {
	// Fetch jobs from the database
	return []BatchJob_t{}
}

func processSlowQuery(db *batchsqlc.Queries, row BatchJob_t, initBlock InitBlock) error {
	// Retrieve the SlowQueryProcessor for the app and op
	processor, exists := slowqueryprocessorfuncs[row.App+row.Op]
	if !exists {
		return fmt.Errorf("no SlowQueryProcessor registered for app %s and op %s", row.App, row.Op)
	}

	// Process the slow query using the registered processor
	status, result, messages, outputFiles, err := processor.DoSlowQuery(initBlock, row.Context, row.Input)
	if err != nil {
		return fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}

	// Update the corresponding batchrows and batches records with the results
	if err := updateSlowQueryResult(db, row, status, result, messages, outputFiles); err != nil {
		return fmt.Errorf("error updating slow query result for app %s and op %s: %v", row.App, row.Op, err)
	}

	return nil
}

func processBatchJob(db *batchsqlc.Queries, row BatchJob_t, initBlock InitBlock) error {
	// Retrieve the BatchProcessor for the app and op
	processor, exists := batchprocessorfuncs[row.App+row.Op]
	if !exists {
		return fmt.Errorf("no BatchProcessor registered for app %s and op %s", row.App, row.Op)
	}

	// Process the batch job using the registered processor
	status, result, messages, blobRows, err := processor.DoBatchJob(initBlock, JSONstr(row.Context), row.Line, JSONstr(row.Input))
	if err != nil {
		return fmt.Errorf("error processing batch job for app %s and op %s: %v", row.App, row.Op, err)
	}

	// Update the corresponding batchrows record with the results
	if err := updateBatchJobResult(db, row, status, result, messages, blobRows); err != nil {
		return fmt.Errorf("error updating batch job result for app %s and op %s: %v", row.App, row.Op, err)
	}

	return nil
}

func updateSlowQueryResult(db *batchsqlc.Queries, row BatchJob_t, status BatchStatus_t, result JSONstr, messages []wscutils.ErrorMessage, outputFiles map[string]string) error {
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
	err := db.UpdateBatchRowsSlowQuery(context.Background(), batchsqlc.UpdateBatchRowsSlowQueryParams{
		Rowid:    int32(row.RowID),
		Status:   batchsqlc.StatusEnum(status),
		Doneat:   pgtype.Timestamp{Time: time.Now()},
		Res:      []byte(result),
		Messages: messagesJSON,
		Doneby:   doneBy,
	})
	if err != nil {
		return err
	}

	// Parse the row.Batch string as a UUID
	batchUUID, err := uuid.Parse(row.Batch)
	if err != nil {
		return fmt.Errorf("invalid batch UUID: %v", err)
	}

	// Convert uuid.UUID to pgtype.UUID
	var pgtypeUUID pgtype.UUID
	err = pgtypeUUID.Scan(batchUUID)
	if err != nil {
		return fmt.Errorf("failed to convert UUID: %v", err)
	}

	// Marshal outputFiles to JSON
	var outputFilesJSON []byte
	if len(outputFiles) > 0 {
		outputFilesJSON, err = json.Marshal(outputFiles)
		if err != nil {
			return fmt.Errorf("failed to marshal outputFiles to JSON: %v", err)
		}
	}

	// Update the batches record with the output files
	err = db.UpdateBatchOutputFiles(context.Background(), batchsqlc.UpdateBatchOutputFilesParams{
		ID:          pgtypeUUID,
		Outputfiles: outputFilesJSON,
	})
	if err != nil {
		return err
	}

	return nil
}

func updateBatchJobResult(db *batchsqlc.Queries, row BatchJob_t, status BatchStatus_t, result JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string) error {
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
	err := db.UpdateBatchRowsBatchJob(context.Background(), batchsqlc.UpdateBatchRowsBatchJobParams{
		Rowid:    int32(row.RowID),
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

func encodeJSONMap(m map[string]string) []byte {
	if len(m) == 0 {
		return nil
	}
	jsonData, err := json.Marshal(m)
	if err != nil {
		// Handle the error appropriately (e.g., log it, return an error)
		return nil
	}
	return jsonData
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		// Handle the error appropriately (e.g., log it, return a default value)
		fmt.Println("Failed to get hostname:", err)
		return "unknown"
	}
	return hostname
}

func main() {
	// Assume db is a *sql.DB connected to your database
	var db *sql.DB
	doneBy := pgtype.Text{}
	err := doneBy.Scan(getHostname())
	if err != nil {
		log.Fatal(err)
	}
	JobManager(db)
}
