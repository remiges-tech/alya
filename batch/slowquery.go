package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

const ALYA_BATCHSTATUS_CACHEDUR_SEC = 100

func (s SlowQuery) RegisterProcessor(app string, op string, p SlowQueryProcessor) error {
	mu.Lock()
	defer mu.Unlock()

	key := app + op
	if _, exists := slowqueryprocessorfuncs[key]; exists {
		return fmt.Errorf("processor for app %s and op %s already registered", app, op)
	}

	slowqueryprocessorfuncs[key] = p
	return nil
}

func (s SlowQuery) Submit(app, op string, inputContext, input JSONstr) (reqID string, err error) {
	// Start a database transaction
	tx, err := s.Db.Begin(context.Background())
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())

	ctx := context.Background()

	batchId, err := uuid.NewUUID()
	if err != nil {
		log.Printf("SlowQuery.Submit uuid.NewUUID failed: %v", err)
		return "", err
	}

	// Use sqlc generated function to insert into batches table
	_, err = s.Queries.InsertIntoBatches(ctx, batchsqlc.InsertIntoBatchesParams{
		ID:      batchId,
		App:     app,
		Op:      op,
		Context: []byte(string(inputContext)),
	})
	if err != nil {
		log.Printf("SlowQuery.Submit InsertIntoBatchesFailed: %v", err)
		return "", err
	}

	// Use sqlc generated function to insert into batchrows table
	err = s.Queries.InsertIntoBatchRows(ctx, batchsqlc.InsertIntoBatchRowsParams{
		Batch: batchId,
		Line:  0,
		Input: json.RawMessage(input),
	})
	if err != nil {
		log.Printf("SlowQuery.Submit InsertIntoBatchRowsFailed: %v", err)
		return "", err
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		log.Printf("SlowQuery.Submit Txn CommitFailed: %v", err)
		return "SlowQuery.Submit Txn CommitFailed", err
	}

	// Return the UUID as reqID and nil for err
	return batchId.String(), nil
}

type BatchStatus_t int

const (
	BatchTryLater BatchStatus_t = iota
	BatchSuccess
	BatchFailed
	BatchAborted
)

// determineBatchStatus converts a batch status from the database or Redis
// to a BatchStatus_t value.
func determineBatchStatus(status batchsqlc.StatusEnum) BatchStatus_t {
	switch status {
	case batchsqlc.StatusEnumSuccess:
		return BatchSuccess
	case batchsqlc.StatusEnumFailed:
		return BatchFailed
	case batchsqlc.StatusEnumAborted:
		return BatchAborted
	default:
		// This includes StatusEnumQueued, StatusEnumInprog, StatusEnumWait, or any other unexpected value.
		return BatchTryLater
	}
}

func (s SlowQuery) Done(reqID string) (status BatchStatus_t, result JSONstr, messages []wscutils.ErrorMessage, err error) {
	// Check REDIS for the status
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", reqID)
	statusVal, err := s.RedisClient.Get(redisKey).Result()
	if err == redis.Nil {
		// Key does not exist in REDIS, check the database
		reqIDUUID, err := uuid.Parse(reqID)
		if err != nil {
			log.Printf("SlowQuery.Done invalid request ID: %v", err)
			return BatchTryLater, "", nil, fmt.Errorf("invalid request ID: %v", err)
		}
		batchStatus, err := s.Queries.GetBatchStatus(context.Background(), reqIDUUID)
		if err != nil {
			log.Printf("SlowQuery.Done GetBatchStatusFailed for request %v: %v", reqID, err)
			return BatchTryLater, "", nil, err // Assuming GetBatchStatus returns an error if not found
		}

		// Convert string to StatusEnum if necessary
		var enumStatus batchsqlc.StatusEnum
		enumStatus.Scan(batchStatus) // Assuming batchStatus is a string, adjust if it's already StatusEnum

		// Determine the BatchStatus_t based on batchStatus
		status := determineBatchStatus(enumStatus)

		// Insert/update REDIS with 100x expiry if not found earlier
		expiry := 100 * ALYA_BATCHSTATUS_CACHEDUR_SEC
		s.RedisClient.Set(redisKey, batchStatus, time.Second*time.Duration(expiry))

		// Return the formatted result, messages, and nil for error
		return status, result, messages, nil

	} else if err != nil {
		return BatchTryLater, "", nil, err
	} else {
		// Key exists in REDIS, determine the action based on its value
		var enumStatus batchsqlc.StatusEnum
		enumStatus.Scan(statusVal) // Convert Redis result to StatusEnum
		status = determineBatchStatus(enumStatus)
	}

	// Format the response based on the status
	return status, result, messages, nil
}

func (s SlowQuery) Abort(reqID string) (err error) {
	// Parse the request ID as a UUID
	reqIDUUID, err := uuid.Parse(reqID)
	if err != nil {
		return fmt.Errorf("invalid request ID: %v", err)
	}

	// Start a transaction
	tx, err := s.Db.Begin(context.Background())
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback(context.Background())

	// Perform SELECT FOR UPDATE on batches and batchrows for the given request ID
	batch, err := s.Queries.GetBatchByID(context.Background(), reqIDUUID)
	if err != nil {
		return fmt.Errorf("failed to get batch by ID: %v", err)
	}

	// Check if the batch status is already aborted, success, or failed
	if batch.Status == batchsqlc.StatusEnumAborted ||
		batch.Status == batchsqlc.StatusEnumSuccess ||
		batch.Status == batchsqlc.StatusEnumFailed {
		return fmt.Errorf("cannot abort batch with status %s", batch.Status)
	}

	// Update the batch status to aborted and set doneat timestamp
	err = s.Queries.UpdateBatchSummary(context.Background(), batchsqlc.UpdateBatchSummaryParams{
		ID:     reqIDUUID,
		Status: batchsqlc.StatusEnumAborted,
		Doneat: pgtype.Timestamp{Time: time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to update batch summary: %v", err)
	}

	// Fetch the pending batchrows records associated with the batch ID
	pendingRows, err := s.Queries.GetPendingBatchRows(context.Background(), reqIDUUID)
	if err != nil {
		return fmt.Errorf("failed to get pending batchrows: %v", err)
	}

	// Extract the rowids from the batchRows
	rowids := make([]int32, len(pendingRows))
	for i, row := range pendingRows {
		rowids[i] = row.Rowid
	}

	// Update the batchrows status to aborted for rows with status queued or inprog
	err = s.Queries.UpdateBatchRowsStatus(context.Background(), batchsqlc.UpdateBatchRowsStatusParams{
		Status:  batchsqlc.StatusEnumAborted,
		Column2: rowids,
	})
	if err != nil {
		return fmt.Errorf("failed to update batchrows status: %v", err)
	}

	// Commit the transaction
	err = tx.Commit(context.Background())
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Set the Redis batch status record to aborted with an expiry time
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", reqID)
	expiry := time.Duration(ALYA_BATCHSTATUS_CACHEDUR_SEC*100) * time.Second
	err = s.RedisClient.Set(redisKey, string(batchsqlc.StatusEnumAborted), expiry).Err()
	if err != nil {
		log.Printf("failed to set Redis batch status: %v", err)
	}

	return nil
}
