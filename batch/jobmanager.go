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

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

const ALYA_BATCHCHUNK_NROWS = 10

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

func init() {
	initblocks = make(map[string]InitBlock)
	initfuncs = make(map[string]Initializer)
	slowqueryprocessorfuncs = make(map[string]SlowQueryProcessor)
	batchprocessorfuncs = make(map[string]BatchProcessor)
}

func RegisterProcessor(app string, op string, p BatchProcessor) error {
	mu.Lock()
	defer mu.Unlock()

	key := app + op
	if _, exists := batchprocessorfuncs[key]; exists {
		return fmt.Errorf("processor for app %s and op %s already registered", app, op)
	}

	batchprocessorfuncs[key] = p
	return nil
}

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
		log.Printf("no initializer registered for app %s", app)
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

func JobManager(pool *pgxpool.Pool, redisClient *redis.Client) {
	for {
		ctx := context.Background()

		// Begin a transaction
		tx, err := pool.Begin(ctx)
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

		var blockNsuccess, blockNfailed, blockNaborted int64
		batchSet := make(map[uuid.UUID]bool)

		// Process each row in the block
		for _, row := range blockOfRows {
			status, err := processRow(txQueries, row)
			if err != nil {
				log.Println("Error processing row:", err)
				continue
			}

			switch status {
			case batchsqlc.StatusEnumSuccess:
				blockNsuccess++
			case batchsqlc.StatusEnumFailed:
				blockNfailed++
			case batchsqlc.StatusEnumAborted:
				blockNaborted++
			}

			if row.Line != 0 {
				// Add the batch ID to the batchList if it's not a slow query
				batchSet[row.Batch] = true

			}
		}

		// Update the counters in the batches table after processing the block
		for batchID := range batchSet {
			fmt.Printf("before updatebatchcounters batchid: %v \n", batchID.String())
			fmt.Printf("before updatebatchcounters blockNsuccess: %v \n", blockNsuccess)
			fmt.Printf("before updatebatchcounters blockNfailed: %v \n", blockNfailed)
			fmt.Printf("before updatebatchcounters blockNaborted: %v \n", blockNaborted)
			err := updateBatchCounters(txQueries, batchID, blockNsuccess, blockNfailed, blockNaborted)
			if err != nil {
				log.Println("Error updating batch counters:", err)
			}
		}

		// Check for completed batches and summarize them
		if err := summarizeCompletedBatches(txQueries, redisClient, batchSet); err != nil {
			log.Println("Error summarizing completed batches:", err)
		}

		tx.Commit(ctx)

		// Close and clean up initblocks
		closeInitBlocks()
	}
}

func updateBatchCounters(db *batchsqlc.Queries, batchID uuid.UUID, nsuccess, nfailed, naborted int64) error {
	fmt.Printf("jobmanager inside updatebatchcounters batchid: %v \n", batchID.String())
	ctx := context.Background()

	err := db.UpdateBatchCounters(ctx, batchsqlc.UpdateBatchCountersParams{
		ID:       batchID,
		Nsuccess: pgtype.Int4{Int32: int32(nsuccess), Valid: true},
		Nfailed:  pgtype.Int4{Int32: int32(nfailed), Valid: true},
		Naborted: pgtype.Int4{Int32: int32(naborted), Valid: true},
	})

	return err
}

func processRow(q *batchsqlc.Queries, row batchsqlc.FetchBlockOfRowsRow) (batchsqlc.StatusEnum, error) {
	fmt.Printf("jobmanager inside processrow\n")
	// Get or create the initblock for the app
	initBlock, err := getOrCreateInitBlock(string(row.App))
	if err != nil {
		log.Printf("error getting or creating initblock for app %s: %v", string(row.App), err)
		return batchsqlc.StatusEnumFailed, err
	}

	// Process the row based on its type (slow query or batch job)
	if row.Line == 0 {
		return processSlowQuery(q, row, initBlock)
	} else {
		return processBatchJob(q, row, initBlock)
	}
}

func summarizeCompletedBatches(q *batchsqlc.Queries, r *redis.Client, batchSet map[uuid.UUID]bool) error {
	fmt.Printf("jobmanager inside summarizecompletedbatches\n")
	for batchID := range batchSet {
		if err := summarizeBatch(q, r, batchID); err != nil {
			log.Println("Error summarizing batch:", batchID, err)
			continue
		}
	}

	return nil
}

func summarizeBatch(q *batchsqlc.Queries, r *redis.Client, batchID uuid.UUID) error {
	fmt.Printf("jobmanager inside summarizebatch\n")
	ctx := context.Background()

	// Fetch the batch record
	batch, err := q.GetBatchByID(ctx, batchID)
	if err != nil {
		return fmt.Errorf("failed to get batch by ID: %v", err)
	}

	// Check if the batch is already summarized
	if !batch.Doneat.Time.IsZero() {
		return nil
	}

	// Fetch pending batchrows records for the batch with status "queued" or "inprog"
	fmt.Printf("jobmanager summarizebatch getpendingbatchrows %v \n", batchID.String())
	pendingRows, err := q.GetPendingBatchRows(ctx, batchID)
	if err != nil {
		return fmt.Errorf("failed to get pending batch rows: %v", err)
	}
	fmt.Printf("jobmanager after getpendingbatchrows pendingrows %v \n", pendingRows)

	// If there are pending rows, the batch is not yet complete
	if len(pendingRows) > 0 {
		return nil
	}

	// Fetch all batchrows records for the batch, sorted by "line"
	batchRows, err := q.GetBatchRowsByBatchIDSorted(ctx, batchID)
	fmt.Printf("jobmanager summarizebatch getbatchrowsbybatchidsorted %v \n", batchID.String())
	fmt.Printf("jobmanager summarizebatch getbatchrowsbybatchidsorted batchrows %v \n", batchRows)
	if err != nil {
		return fmt.Errorf("failed to get batch rows sorted: %v", err)
	}

	// Create temporary files for each unique logical file in blobrows
	tmpFiles := make(map[string]*os.File)
	for _, row := range batchRows {
		var blobRows map[string]string
		if err := json.Unmarshal(row.Blobrows, &blobRows); err != nil {
			return fmt.Errorf("failed to unmarshal blobrows: %v", err)
		}

		for logicalFile := range blobRows {
			if _, exists := tmpFiles[logicalFile]; !exists {
				file, err := os.CreateTemp("", logicalFile)
				if err != nil {
					return fmt.Errorf("failed to create temporary file: %v", err)
				}
				tmpFiles[logicalFile] = file
			}
		}
	}

	// Append blobrows strings to the appropriate temporary files
	for _, row := range batchRows {
		var blobRows map[string]string
		if err := json.Unmarshal(row.Blobrows, &blobRows); err != nil {
			return fmt.Errorf("failed to unmarshal blobrows: %v", err)
		}

		for logicalFile, content := range blobRows {
			if _, err := tmpFiles[logicalFile].WriteString(content + "\n"); err != nil {
				return fmt.Errorf("failed to write to temporary file: %v", err)
			}
		}
	}

	// Close all temporary files
	for _, file := range tmpFiles {
		if err := file.Close(); err != nil {
			return fmt.Errorf("failed to close temporary file: %v", err)
		}
	}

	// Move temporary files to the object store and update outputfiles
	outputFiles := make(map[string]string)
	for logicalFile, file := range tmpFiles {
		objectID, err := moveToObjectStore(file.Name())
		if err != nil {
			return fmt.Errorf("failed to move file to object store: %v", err)
		}
		outputFiles[logicalFile] = objectID
	}

	// Update the batches record with summarized information
	outputFilesJSON, err := json.Marshal(outputFiles)
	if err != nil {
		return fmt.Errorf("failed to marshal output files: %v", err)
	}

	status := batchsqlc.StatusEnumSuccess
	if batch.Nfailed.Int32 > 0 {
		status = batchsqlc.StatusEnumFailed
	}

	fmt.Printf("jobmanager summarizebatch updatebatchsummary batch: %v \n", batch)
	err = q.UpdateBatchSummary(ctx, batchsqlc.UpdateBatchSummaryParams{
		ID:          batchID,
		Status:      status,
		Doneat:      pgtype.Timestamp{Time: time.Now(), Valid: true},
		Outputfiles: outputFilesJSON,
		Nsuccess:    batch.Nsuccess,
		Nfailed:     batch.Nfailed,
		Naborted:    batch.Naborted,
	})
	if err != nil {
		return fmt.Errorf("failed to update batch summary: %v", err)
	}

	// update status in redis
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", batchID)
	expiry := time.Duration(ALYA_BATCHSTATUS_CACHEDUR_SEC*100) * time.Second
	_, err = r.Set(redisKey, string(status), expiry).Result()
	if err != nil {
		return fmt.Errorf("failed to update status in redis: %v", err)
	}

	return nil
}

func moveToObjectStore(filePath string) (string, error) {
	// Implement the logic to move the file to the object store
	// and return the object ID
	// return "", fmt.Errorf("moveToObjectStore not implemented")
	return "", nil
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

func processSlowQuery(db *batchsqlc.Queries, row batchsqlc.FetchBlockOfRowsRow, initBlock InitBlock) (batchsqlc.StatusEnum, error) {
	log.Printf("processing slow query for app %s and op %s", row.App, row.Op)
	// Retrieve the SlowQueryProcessor for the app and op
	processor, exists := slowqueryprocessorfuncs[string(row.App)+row.Op]
	if !exists {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("no SlowQueryProcessor registered for app %s and op %s", row.App, row.Op)
	}

	// Process the slow query using the registered processor
	status, result, messages, outputFiles, err := processor.DoSlowQuery(initBlock, JSONstr(string(row.Context)), JSONstr(string(row.Input)))
	if err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing slow query for app %s and op %s: %v", row.App, row.Op, err)
	}

	// Update the corresponding batchrows and batches records with the results
	if err := updateSlowQueryResult(db, row, status, result, messages, outputFiles); err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error updating slow query result for app %s and op %s: %v", row.App, row.Op, err)
	}

	return status, nil
}

func processBatchJob(db *batchsqlc.Queries, row batchsqlc.FetchBlockOfRowsRow, initBlock InitBlock) (batchsqlc.StatusEnum, error) {
	// Retrieve the BatchProcessor for the app and op
	processor, exists := batchprocessorfuncs[string(row.App)+row.Op]
	if !exists {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("no BatchProcessor registered for app %s and op %s", row.App, row.Op)
	}

	// Process the batch job using the registered processor
	status, result, messages, blobRows, err := processor.DoBatchJob(initBlock, JSONstr(string(row.Context)), int(row.Line), JSONstr(string(row.Input)))
	if err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error processing batch job for app %s and op %s: %v", row.App, row.Op, err)
	}

	// Update the corresponding batchrows record with the results
	if err := updateBatchJobResult(db, row, status, result, messages, blobRows); err != nil {
		return batchsqlc.StatusEnumFailed, fmt.Errorf("error updating batch job result for app %s and op %s: %v", row.App, row.Op, err)
	}

	return status, nil
}

func updateSlowQueryResult(db *batchsqlc.Queries, row batchsqlc.FetchBlockOfRowsRow, status batchsqlc.StatusEnum, result JSONstr, messages []wscutils.ErrorMessage, outputFiles map[string]string) error {
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

	// Marshal outputFiles to JSON
	var outputFilesJSON []byte
	if len(outputFiles) > 0 {
		outputFilesJSON, err = json.Marshal(outputFiles)
		if err != nil {
			return fmt.Errorf("failed to marshal outputFiles to JSON: %v", err)
		}
	}

	// Update the batches record based on the status of the slow query
	if status == batchsqlc.StatusEnumSuccess {
		err = db.UpdateBatchSummary(context.Background(), batchsqlc.UpdateBatchSummaryParams{
			ID:          row.Batch,
			Status:      batchsqlc.StatusEnumSuccess,
			Doneat:      pgtype.Timestamp{Time: time.Now()},
			Outputfiles: outputFilesJSON,
			Nsuccess:    pgtype.Int4{Int32: 1, Valid: true},
			Nfailed:     pgtype.Int4{Int32: 0, Valid: true},
			Naborted:    pgtype.Int4{Int32: 0, Valid: true},
		})
		if err != nil {
			return err
		}
	} else if status == batchsqlc.StatusEnumFailed {
		err = db.UpdateBatchSummary(context.Background(), batchsqlc.UpdateBatchSummaryParams{
			ID:          row.Batch,
			Status:      batchsqlc.StatusEnumFailed,
			Doneat:      pgtype.Timestamp{Time: time.Now()},
			Outputfiles: outputFilesJSON,
			Nsuccess:    pgtype.Int4{Int32: 0, Valid: true},
			Nfailed:     pgtype.Int4{Int32: 1, Valid: true},
			Naborted:    pgtype.Int4{Int32: 0, Valid: true},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func updateBatchJobResult(db *batchsqlc.Queries, row batchsqlc.FetchBlockOfRowsRow, status batchsqlc.StatusEnum, result JSONstr, messages []wscutils.ErrorMessage, blobRows map[string]string) error {
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

func getCompletedBatches(pool *pgxpool.Pool) ([]uuid.UUID, error) {
	ctx := context.Background()

	// Create a new Queries instance using the pool
	q := batchsqlc.New(pool)

	// Retrieve batches with status "success", "failed", or "aborted"
	batches, err := q.GetCompletedBatches(ctx)
	if err != nil {
		return nil, err
	}

	return batches, nil
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
