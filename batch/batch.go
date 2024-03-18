package batch

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
)

type Batch struct {
	Db          *pgxpool.Pool
	Queries     batchsqlc.Querier
	RedisClient *redis.Client
}

func (jm *JobManager) RegisterProcessorBatch(app string, op string, p BatchProcessor) error {
	mu.Lock()
	defer mu.Unlock()

	key := app + op
	if _, exists := jm.batchprocessorfuncs[key]; exists {
		return fmt.Errorf("processor for app %s and op %s already registered", app, op)
	}

	jm.batchprocessorfuncs[key] = p
	return nil
}

func (jm *JobManager) RegisterProcessorSlowQuery(app string, op string, s SlowQueryProcessor) error {
	mu.Lock()
	defer mu.Unlock()

	key := app + op
	if _, exists := jm.slowqueryprocessorfuncs[key]; exists {
		return fmt.Errorf("processor for app %s and op %s already registered", app, op)
	}

	jm.slowqueryprocessorfuncs[key] = s
	return nil
}

func (jm *JobManager) BatchSubmit(app, op string, batchctx JSONstr, batchInput []batchsqlc.InsertIntoBatchRowsParams, waitabit bool) (batchID string, err error) {
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

	// Insert a record into the batches table
	_, err = jm.Queries.InsertIntoBatches(context.Background(), batchsqlc.InsertIntoBatchesParams{
		ID:      batchUUID,
		App:     app,
		Op:      op,
		Context: []byte(batchctx),
		Status:  status,
	})
	if err != nil {
		return "", err
	}

	// Insert records into the batchrows table
	// TODO: do it in bulk
	for _, input := range batchInput {
		input.Batch = batchUUID
		err := jm.Queries.InsertIntoBatchRows(context.Background(), input)
		if err != nil {
			return "", err
		}
	}

	// Commit the transaction
	err = tx.Commit(context.Background())
	if err != nil {
		return "", err
	}

	return batchUUID.String(), nil
}

func (jm *JobManager) BatchDone(batchID string) (status batchsqlc.StatusEnum, batchOutput []batchsqlc.FetchBatchRowsDataRow, outputFiles map[string]string, nsuccess, nfailed, naborted int, err error) {
	var batch batchsqlc.Batch
	// Check REDIS for the batch status
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", batchID)
	statusVal, err := jm.RedisClient.Get(redisKey).Result()
	if err == redis.Nil {
		// Key does not exist in REDIS, check the database
		batch, err := jm.Queries.GetBatchByID(context.Background(), uuid.MustParse(batchID))
		if err != nil {
			return batchsqlc.StatusEnumWait, nil, nil, 0, 0, 0, err
		}
		status = batch.Status

		// Update REDIS with batches.status and an expiry duration
		expiry := time.Duration(ALYA_BATCHSTATUS_CACHEDUR_SEC*100) * time.Second
		err = jm.RedisClient.Set(redisKey, string(batch.Status), expiry).Err()
		if err != nil {
			// Log the error, but continue processing
			log.Printf("Error setting REDIS key %s: %v", redisKey, err)
		}
	} else if err != nil {
		return batchsqlc.StatusEnumWait, nil, nil, 0, 0, 0, err
	} else {
		// Key exists in REDIS, use the status value from REDIS
		status = batchsqlc.StatusEnum(statusVal)
	}

	switch status {
	case batchsqlc.StatusEnumAborted, batchsqlc.StatusEnumFailed, batchsqlc.StatusEnumSuccess:
		// Fetch batch rows data
		batchOutput, err = jm.Queries.FetchBatchRowsData(context.Background(), uuid.MustParse(batchID))
		if err != nil {
			return status, nil, nil, 0, 0, 0, err
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
	fmt.Printf("batch.abort inside abort\n")
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

	// Perform SELECT FOR UPDATE on batches and batchrows for the given batch ID
	fmt.Printf("batch.abort before getbatchbyid\n")
	batch, err := jm.Queries.GetBatchByID(context.Background(), batchUUID)
	if err == sql.ErrNoRows {
		return "", 0, 0, 0, fmt.Errorf("batch not found: %v", err)
	}
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("failed to get batch by ID: %v", err)
	}
	fmt.Printf("batch.abort after getbatchbyid\n")

	// Check if the batch status is already aborted, success, or failed
	if batch.Status == batchsqlc.StatusEnumAborted ||
		batch.Status == batchsqlc.StatusEnumSuccess ||
		batch.Status == batchsqlc.StatusEnumFailed {
		return batch.Status, int(batch.Nsuccess.Int32), int(batch.Nfailed.Int32), int(batch.Naborted.Int32), nil
	}

	// Fetch the pending batchrows records associated with the batch ID
	fmt.Printf("batch.abort before getpendingbatchrows batchuuid: %v \n", batchUUID.String())
	pendingRows, err := jm.Queries.GetPendingBatchRows(context.Background(), batchUUID)
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
	rowids := make([]int32, len(pendingRows))
	for i, row := range pendingRows {
		rowids[i] = row.Rowid
	}

	// Update the batchrows status to aborted for rows with status queued or inprog
	fmt.Printf("batch.abort before updatebatchrowsstatus rowids %v:  \n", rowids)
	err = jm.Queries.UpdateBatchRowsStatus(context.Background(), batchsqlc.UpdateBatchRowsStatusParams{
		Status:  batchsqlc.StatusEnumAborted,
		Column2: rowids,
	})
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("failed to update batchrows status: %v", err)
	}

	// Update the batch status to aborted and set doneat timestamp
	fmt.Printf("batch.abort before updatebatchsummary\n")
	err = jm.Queries.UpdateBatchSummaryOnAbort(context.Background(), batchsqlc.UpdateBatchSummaryOnAbortParams{
		ID:       batchUUID,
		Status:   batchsqlc.StatusEnumAborted,
		Doneat:   pgtype.Timestamp{Time: time.Now()},
		Naborted: pgtype.Int4{Int32: int32(len(pendingRows)), Valid: true},
	})
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("failed to update batch summary: %v", err)
	}

	// Commit the transaction
	fmt.Printf("batch.abort before tx.commit")
	err = tx.Commit(context.Background())
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Set the Redis batch status record to aborted with an expiry time
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", batchID)
	expiry := time.Duration(ALYA_BATCHSTATUS_CACHEDUR_SEC*100) * time.Second
	err = jm.RedisClient.Set(redisKey, string(batchsqlc.StatusEnumAborted), expiry).Err()
	if err != nil {
		log.Printf("failed to set Redis batch status: %v", err)
	}

	return batchsqlc.StatusEnumAborted, int(batch.Nsuccess.Int32), int(batch.Nfailed.Int32), len(pendingRows), nil
}

func (jm *JobManager) BatchAppend(batchID string, batchinput []batchsqlc.InsertIntoBatchRowsParams, waitabit bool) (nrows int, err error) {
	// Check if the batch record exists in the batches table
	batch, err := jm.Queries.GetBatchByID(context.Background(), uuid.MustParse(batchID))
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

	// Insert records into the batchrows table
	for _, input := range batchinput {
		if input.Line <= 0 {
			return 0, fmt.Errorf("invalid line number: %d", input.Line)
		}

		err := jm.Queries.InsertIntoBatchRows(context.Background(), input)
		if err != nil {
			return 0, fmt.Errorf("failed to insert batch row: %v", err)
		}
	}

	// Update the batch status to "queued" if waitabit is false
	if !waitabit {
		err = jm.Queries.UpdateBatchStatus(context.Background(), batchsqlc.UpdateBatchStatusParams{
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
