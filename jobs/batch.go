package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
)

type Batch struct {
	Db          *pgxpool.Pool
	Queries     batchsqlc.Querier
	RedisClient *redis.Client
}

type BatchOutput_t struct {
	Line     int
	Status   BatchStatus_t
	Res      JSONstr
	Messages JSONstr
}

// RegisterProcessorBatch allows applications to register a processing function for a specific batch operation type.
// The processing function implements the BatchProcessor interface.
// Each (app, op) combination can only have one registered processor.
// Attempting to register a second processor for the same combination will result in an error.
// The 'op' parameter is case-insensitive and will be converted to lowercase before registration.
func (jm *JobManager) RegisterProcessorBatch(app string, op string, p BatchProcessor) error {
	// Convert op to lowercase before inserting into the database
	op = strings.ToLower(op)

	key := app + op
	_, exists := jm.batchprocessorfuncs[key]
	if exists {
		return fmt.Errorf("%w: app=%s, op=%s", ErrProcessorAlreadyRegistered, app, op)
	}
	jm.batchprocessorfuncs[key] = p // Add this line to store the processor
	return nil
}

// BatchSubmit submits a new batch for processing.
// It generates a unique batch ID, inserts a record into the "batches" table, and inserts multiple records
// into the "batchrows" table corresponding to the provided batch input. The batch is then picked up and processed by the
// JobManager's worker goroutines spawned by Run().
// Note that the operation or task to be performed on each batch row (value is converted to lowercase).
// The 'waitabit' parameter determines the initial status of the batch. If 'waitabit' is true, the batch
// status will be set to 'wait', indicating that the batch should be held back from immediate processing. If
// 'waitabit' is false, the batch status will be set to 'queued', making it available for processing.
func (jm *JobManager) BatchSubmit(app, op string, batchctx JSONstr, batchInput []BatchInput_t, waitabit bool) (batchID string, err error) {
	// Generate a unique batch ID
	batchUUID, err := uuid.NewUUID()

	// Start a transaction
	tx, err := jm.Db.Begin(context.Background())
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())

	// Set the batch status based on waitabit
	status := batchsqlc.StatusEnumQueued
	if waitabit {
		status = batchsqlc.StatusEnumWait
	}

	// Convert op to lowercase before inserting into the database
	op = strings.ToLower(op)

	// Create transaction-bound queries
	txQueries := batchsqlc.New(tx)

	// Insert a record into the batches table
	_, err = txQueries.InsertIntoBatches(context.Background(), batchsqlc.InsertIntoBatchesParams{
		ID:      batchUUID,
		App:     app,
		Op:      op,
		Context: []byte(batchctx.String()),
		Status:  status,
		Reqat:   pgtype.Timestamp{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return "", err
	}

	// Insert records into the batchrows table
	batchRowsParam := batchsqlc.BulkInsertIntoBatchRowsParams{
		Batch: make([]uuid.UUID, len(batchInput)),
		Line:  make([]int32, len(batchInput)),
		Input: make([][]byte, len(batchInput)),
		Reqat: make([]pgtype.Timestamp, len(batchInput)),
	}
	for i, input := range batchInput {
		batchRowsParam.Batch[i] = batchUUID
		batchRowsParam.Line[i] = int32(input.Line)
		batchRowsParam.Input[i] = []byte(input.Input.String())
		batchRowsParam.Reqat[i] = pgtype.Timestamp{Time: time.Now(), Valid: true}
	}
	_, err = txQueries.BulkInsertIntoBatchRows(context.Background(), batchRowsParam)
	if err != nil {
		return "", err
	}

	// Commit the transaction
	err = tx.Commit(context.Background())
	if err != nil {
		return "", err
	}

	return batchUUID.String(), nil
}

func (jm *JobManager) BatchDone(batchID string) (status batchsqlc.StatusEnum, batchOutput []BatchOutput_t, outputFiles map[string]string, nsuccess, nfailed, naborted int, err error) {
	var batch batchsqlc.Batch
	// Check REDIS for the batch status
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", batchID)
	statusVal, err := jm.RedisClient.Get(context.Background(), redisKey).Result()
	if err == redis.Nil {
		// Key does not exist in REDIS, check the database
		batch, err = jm.Queries.GetBatchByID(context.Background(), uuid.MustParse(batchID))
		if err != nil {
			return batchsqlc.StatusEnumWait, nil, nil, 0, 0, 0, err
		}
		status = batch.Status

		// Update REDIS with batches.status and an expiry duration
		expirySec := jm.Config.BatchStatusCacheDurSec
		if status == batchsqlc.StatusEnumSuccess || status == batchsqlc.StatusEnumFailed || status == batchsqlc.StatusEnumAborted {
			// final status, set expiry to 100 times the cache duration
			expirySec = 100 * jm.Config.BatchStatusCacheDurSec
		}
		err = updateStatusInRedis(jm.RedisClient, uuid.MustParse(batchID), status, expirySec)
		if err != nil {
			log.Printf("Error setting REDIS key %s: %v", redisKey, err)
		}
	} else if err != nil {
		return batchsqlc.StatusEnumWait, nil, nil, 0, 0, 0, err
	} else {
		// Key exists in REDIS, use the status value from REDIS
		status = batchsqlc.StatusEnum(statusVal)
		// Fetch the batch record from the database
		batch, err = jm.Queries.GetBatchByID(context.Background(), uuid.MustParse(batchID))
		if err != nil {
			return batchsqlc.StatusEnumWait, nil, nil, 0, 0, 0, err
		}
	}

	switch status {
	case batchsqlc.StatusEnumAborted, batchsqlc.StatusEnumFailed, batchsqlc.StatusEnumSuccess:
		// Fetch batch rows data
		batchRowsData, err := jm.Queries.FetchBatchRowsForBatchDone(context.Background(), uuid.MustParse(batchID))
		if err != nil {
			return status, nil, nil, 0, 0, 0, err
		}

		// Convert batchRowsData to BatchOutput_t
		batchOutput = make([]BatchOutput_t, len(batchRowsData))
		for i, row := range batchRowsData {
			res, err := NewJSONstr(string(row.Res))
			if err != nil {
				return status, nil, nil, 0, 0, 0, fmt.Errorf("failed to parse Res JSON for line %d: %v", row.Line, err)
			}
			messages, err := NewJSONstr(string(row.Messages))
			if err != nil {
				return status, nil, nil, 0, 0, 0, fmt.Errorf("failed to parse Messages JSON for line %d: %v", row.Line, err)
			}
			batchOutput[i] = BatchOutput_t{
				Line:     int(row.Line),
				Status:   mapStatusEnum(row.Status),
				Res:      res,
				Messages: messages,
			}
		}

		// Fetch output files from the batches table
		outputFiles = make(map[string]string)
		json.Unmarshal(batch.Outputfiles, &outputFiles)

		// Fetch batch counters from the batches table
		nsuccess = int(batch.Nsuccess.Int32)
		nfailed = int(batch.Nfailed.Int32)
		naborted = int(batch.Naborted.Int32)

	case batchsqlc.StatusEnumQueued, batchsqlc.StatusEnumInprog, batchsqlc.StatusEnumWait:
		// Return with status indicating to try later
		return status, nil, nil, 0, 0, 0, nil
	}

	return status, batchOutput, outputFiles, nsuccess, nfailed, naborted, nil
}

func (jm *JobManager) BatchAbort(batchID string) (status batchsqlc.StatusEnum, nsuccess, nfailed, naborted int, err error) {
	fmt.Printf("jobs.abort inside abort\n")
	// Parse the batch ID as a UUID
	batchUUID, err := uuid.Parse(batchID)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("invalid batch ID: %v", err)
	}

	// Start a transaction
	tx, err := jm.Db.Begin(context.Background())
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback(context.Background())

	queries := batchsqlc.New(tx)

	// Perform SELECT FOR UPDATE on batches and batchrows for the given batch ID
	fmt.Printf("jobs.abort before getbatchbyid\n")
	batch, err := queries.GetBatchByID(context.Background(), batchUUID)
	if err == sql.ErrNoRows {
		return "", 0, 0, 0, fmt.Errorf("batch not found: %v", err)
	}
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("failed to get batch by ID: %v", err)
	}
	fmt.Printf("jobs.abort after getbatchbyid\n")

	// Check if the batch status is already aborted, success, or failed
	if batch.Status == batchsqlc.StatusEnumAborted ||
		batch.Status == batchsqlc.StatusEnumSuccess ||
		batch.Status == batchsqlc.StatusEnumFailed {
		return batch.Status, int(batch.Nsuccess.Int32), int(batch.Nfailed.Int32), int(batch.Naborted.Int32), nil
	}

	// Fetch the pending batchrows records associated with the batch ID
	fmt.Printf("jobs.abort before getpendingbatchrows batchuuid: %v \n", batchUUID.String())
	pendingRows, err := queries.GetPendingBatchRows(context.Background(), batchUUID)
	if len(pendingRows) == 0 {
		return "", 0, 0, 0, fmt.Errorf("no pending rows found for batch %s", batchID)
	}
	if err == sql.ErrNoRows {
		return "", 0, 0, 0, fmt.Errorf("batch not found: %v", err)
	}
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("failed to get pending batchrows: %v", err)
	}

	// Extract the rowids from the batchRows
	rowids := make([]int64, len(pendingRows))
	for i, row := range pendingRows {
		rowids[i] = row.Rowid
	}

	// Update the batchrows status to aborted for rows with status queued or inprog
	fmt.Printf("jobs.abort before updatebatchrowsstatus rowids %v:  \n", rowids)
	err = queries.UpdateBatchRowsStatus(context.Background(), batchsqlc.UpdateBatchRowsStatusParams{
		Status:  batchsqlc.StatusEnumAborted,
		Column2: rowids,
	})
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("failed to update batchrows status: %v", err)
	}

	// Fetch the updated batchrows records for the batch
	updatedBatchRows, err := queries.GetBatchRowsByBatchID(context.Background(), batchUUID)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("failed to get updated batchrows: %v", err)
	}

	// Count the number of rows with each status
	var successCount, failedCount, abortedCount int
	for _, row := range updatedBatchRows {
		switch row.Status {
		case batchsqlc.StatusEnumSuccess:
			successCount++
		case batchsqlc.StatusEnumFailed:
			failedCount++
		case batchsqlc.StatusEnumAborted:
			abortedCount++
		}
	}

	// Update the batch status to aborted and set doneat timestamp
	fmt.Printf("jobs.abort before updatebatchsummary\n")
	err = queries.UpdateBatchSummary(context.Background(), batchsqlc.UpdateBatchSummaryParams{
		ID:       batchUUID,
		Status:   batchsqlc.StatusEnumAborted,
		Doneat:   pgtype.Timestamp{Time: time.Now(), Valid: true},
		Nsuccess: pgtype.Int4{Int32: int32(successCount), Valid: true},
		Nfailed:  pgtype.Int4{Int32: int32(failedCount), Valid: true},
		Naborted: pgtype.Int4{Int32: int32(abortedCount), Valid: true},
	})
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("failed to update batch summary: %v", err)
	}

	// Commit the transaction
	fmt.Printf("jobs.abort before tx.commit")
	err = tx.Commit(context.Background())
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Update status in Redis
	err = updateStatusInRedis(jm.RedisClient, batchUUID, batchsqlc.StatusEnumAborted, 100*jm.Config.BatchStatusCacheDurSec)
	if err != nil {
		log.Printf("failed to update status in Redis: %v", err)
	}
	return batchsqlc.StatusEnumAborted, successCount, failedCount, abortedCount, nil
}

func (jm *JobManager) BatchAppend(batchID string, batchinput []BatchInput_t, waitabit bool) (nrows int, err error) {
	// Parse batch UUID
	batchUUID, err := uuid.Parse(batchID)
	if err != nil {
		return 0, fmt.Errorf("invalid batch ID: %v", err)
	}

	// Check if the batch record exists in the batches table
	batch, err := jm.Queries.GetBatchByID(context.Background(), batchUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("batch not found: %v", err)
		}
		return 0, fmt.Errorf("failed to get batch by ID: %v", err)
	}

	// Check if the batch status is "wait"
	if batch.Status != batchsqlc.StatusEnumWait {
		return 0, fmt.Errorf("batch status must be 'wait' to append rows")
	}

	// Start a transaction
	tx, err := jm.Db.Begin(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback(context.Background())

	// Create transaction-bound queries
	txQueries := batchsqlc.New(tx)

	// Insert records into the batchrows table
	for _, input := range batchinput {
		if input.Line <= 0 {
			return 0, fmt.Errorf("invalid line number: %d", input.Line)
		}

		// Convert BatchInput_t to InsertIntoBatchRowsParams
		dbParams := batchsqlc.InsertIntoBatchRowsParams{
			Batch: batchUUID,
			Line:  int32(input.Line),
			Input: []byte(input.Input.String()),
			Reqat: pgtype.Timestamp{Time: time.Now(), Valid: true},
		}

		err := txQueries.InsertIntoBatchRows(context.Background(), dbParams)
		if err != nil {
			return 0, fmt.Errorf("failed to insert batch row: %v", err)
		}
	}

	// Update the batch status to "queued" if waitabit is false
	if !waitabit {
		err = txQueries.UpdateBatchStatus(context.Background(), batchsqlc.UpdateBatchStatusParams{
			ID:     uuid.MustParse(batchID),
			Status: batchsqlc.StatusEnumQueued,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to update batch status: %v", err)
		}
	}

	// Commit the transaction
	err = tx.Commit(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Get the total count of rows in batchrows for the batch
	batchRows, err := jm.Queries.GetBatchRowsByBatchID(context.Background(), uuid.MustParse(batchID))
	if err != nil {
		return 0, fmt.Errorf("failed to get batch rows: %v", err)
	}
	nrows = len(batchRows)

	return int(nrows), nil
}

func (jm *JobManager) WaitOff(batchID string) (string, int, error) {
	// Parse the batch ID as a UUID
	batchUUID, err := uuid.Parse(batchID)
	if err != nil {
		return "", 0, fmt.Errorf("invalid batch ID: %v", err)
	}

	// Start a transaction
	tx, err := jm.Db.Begin(context.Background())
	if err != nil {
		return "", 0, fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback(context.Background())

	// Perform SELECT FOR UPDATE on the batches table
	batch, err := jm.Queries.GetBatchByID(context.Background(), batchUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", 0, fmt.Errorf("batch not found: %v", err)
		}
		return "", 0, fmt.Errorf("failed to get batch by ID: %v", err)
	}

	// Check if the batch status is already "queued"
	if batch.Status == batchsqlc.StatusEnumQueued {
		// Get the total count of rows in batchrows for the batch
		batchRows, err := jm.Queries.GetBatchRowsCount(context.Background(), batchUUID)
		if err != nil {
			return "", 0, fmt.Errorf("failed to get batch rows: %v", err)
		}

		// No need to update the status, return success
		return batchID, int(batchRows), nil
	}

	// Check if the batch status is "wait"
	if batch.Status != batchsqlc.StatusEnumWait {
		return "", 0, fmt.Errorf("batch status must be 'wait' to change to 'queued'")
	}

	// Update the batch status to "queued"
	err = jm.Queries.UpdateBatchStatus(context.Background(), batchsqlc.UpdateBatchStatusParams{
		ID:     batchUUID,
		Status: batchsqlc.StatusEnumQueued,
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to update batch status: %v", err)
	}

	// Get the total count of rows in batchrows for the batch
	nrows, err := jm.Queries.GetBatchRowsCount(context.Background(), batchUUID)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get batch rows count: %v", err)
	}

	// Commit the transaction
	err = tx.Commit(context.Background())
	if err != nil {
		return "", 0, fmt.Errorf("failed to commit transaction: %v", err)
	}

	return batchID, int(nrows), nil
}

func GetBatchStatusRedisKey(batchID string) string {
	return fmt.Sprintf("batch:%s:status", batchID)
}

func mapStatusEnum(status batchsqlc.StatusEnum) BatchStatus_t {
	switch status {
	case batchsqlc.StatusEnumWait:
		return BatchWait
	case batchsqlc.StatusEnumQueued:
		return BatchQueued
	case batchsqlc.StatusEnumInprog:
		return BatchInProgress
	case batchsqlc.StatusEnumSuccess:
		return BatchSuccess
	case batchsqlc.StatusEnumFailed:
		return BatchFailed
	case batchsqlc.StatusEnumAborted:
		return BatchAborted
	default:
		return BatchTryLater
	}
}
