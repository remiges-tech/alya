package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
	"github.com/remiges-tech/alya/wscutils"
)

// ErrProcessorAlreadyRegistered is returned when attempting to register a second processor
// for the same (app, op) combination.
var ErrProcessorAlreadyRegistered = errors.New("processor already registered for this app and operation")

// RegisterProcessorSlowQuery allows applications to register a processing function for a specific operation type.
// The processing function implements the SlowQueryProcessor interface.
// Each (app, op) combination can only have one registered processor.
// Attempting to register a second processor for the same combination will result in an error.
// The 'op' parameter is case-insensitive and will be converted to lowercase before registration.
func (jm *JobManager) RegisterProcessorSlowQuery(app string, op string, p SlowQueryProcessor) error {
	key := app + op
	_, exists := jm.slowqueryprocessorfuncs[key]
	if exists {
		return fmt.Errorf("%w: app=%s, op=%s", ErrProcessorAlreadyRegistered, app, op)
	}
	jm.slowqueryprocessorfuncs[key] = p // Add this line to store the processor
	return nil
}

func (jm *JobManager) SlowQuerySubmit(app, op string, inputContext, input JSONstr) (reqID string, err error) {
	// Start a database transaction
	tx, err := jm.Db.Begin(context.Background())
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

	// Convert op to lowercase before inserting into the database
	op = strings.ToLower(op)

	// Use sqlc generated function to insert into batches table
	_, err = jm.Queries.InsertIntoBatches(ctx, batchsqlc.InsertIntoBatchesParams{
		ID:      batchId,
		App:     app,
		Op:      op,
		Context: []byte(inputContext.String()),
		Status:  batchsqlc.StatusEnumQueued,
		Reqat:   pgtype.Timestamp{Time: time.Now(), Valid: true},
	})
	if err != nil {
		log.Printf("SlowQuery.Submit InsertIntoBatchesFailed: %v", err)
		return "", err
	}

	// Use sqlc generated function to insert into batchrows table
	err = jm.Queries.InsertIntoBatchRows(ctx, batchsqlc.InsertIntoBatchRowsParams{
		Batch: batchId,
		Line:  0,
		Input: []byte(input.String()),
		Reqat: pgtype.Timestamp{Time: time.Now(), Valid: true},
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
	BatchWait
	BatchQueued
	BatchInProgress
)

// determineBatchStatus converts a batch status from the database or Redis
// to a BatchStatus_t value.
func getBatchStatus(status batchsqlc.StatusEnum) BatchStatus_t {
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

func (jm *JobManager) SlowQueryDone(reqID string) (status BatchStatus_t, result JSONstr, messages []wscutils.ErrorMessage, err error) {
	// Check REDIS for the status
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", reqID)
	statusVal, err := jm.RedisClient.Get(context.Background(), redisKey).Result()
	if err == redis.Nil {
		// Key does not exist in REDIS, check the database
		reqIDUUID, err := uuid.Parse(reqID)
		if err != nil {
			log.Printf("SlowQuery.Done invalid request ID: %v", err)
			result, _ := NewJSONstr("")
			return BatchTryLater, result, nil, fmt.Errorf("invalid request ID: %v", err)
		}
		batchStatus, err := jm.Queries.GetBatchStatus(context.Background(), reqIDUUID)
		if err != nil {
			log.Printf("SlowQuery.Done GetBatchStatusFailed for request %v: %v", reqID, err)
			result, _ := NewJSONstr("")
			return BatchTryLater, result, nil, err // Assuming GetBatchStatus returns an error if not found
		}

		// Convert string to StatusEnum if necessary
		var enumStatus batchsqlc.StatusEnum
		enumStatus.Scan(batchStatus) // Assuming batchStatus is a string, adjust if it's already StatusEnum

		// Determine the BatchStatus_t based on batchStatus
		status := getBatchStatus(enumStatus)

		// Insert/update REDIS with 100x expiry if not found earlier
		expirySec := jm.Config.BatchStatusCacheDurSec
		if status == BatchSuccess || status == BatchFailed || status == BatchAborted {
			expirySec = 100 * jm.Config.BatchStatusCacheDurSec
		}
		updateStatusInRedis(jm.RedisClient, reqIDUUID, batchStatus, expirySec)

		// Return the formatted result, messages, and nil for error
		return status, result, messages, nil

	} else if err != nil {
		result, _ := NewJSONstr("")
		return BatchTryLater, result, nil, err
	} else {
		// Key exists in REDIS, determine the action based on its value
		var enumStatus batchsqlc.StatusEnum
		enumStatus.Scan(statusVal) // Convert Redis result to StatusEnum
		status = getBatchStatus(enumStatus)
	}

	// Format the response based on the status
	return status, result, messages, nil
}

func (jm *JobManager) SlowQueryAbort(reqID string) (err error) {
	// Parse the request ID as a UUID
	reqIDUUID, err := uuid.Parse(reqID)
	if err != nil {
		return fmt.Errorf("invalid request ID: %v", err)
	}

	// Start a transaction
	tx, err := jm.Db.Begin(context.Background())
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback(context.Background())

	// Perform SELECT FOR UPDATE on batches and batchrows for the given request ID
	batch, err := jm.Queries.GetBatchByID(context.Background(), reqIDUUID)
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
	err = jm.Queries.UpdateBatchSummary(context.Background(), batchsqlc.UpdateBatchSummaryParams{
		ID:     reqIDUUID,
		Status: batchsqlc.StatusEnumAborted,
		Doneat: pgtype.Timestamp{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to update batch summary: %v", err)
	}

	// Fetch the pending batchrows records associated with the batch ID
	pendingRows, err := jm.Queries.GetPendingBatchRows(context.Background(), reqIDUUID)
	if err != nil {
		return fmt.Errorf("failed to get pending batchrows: %v", err)
	}

	// Extract the rowids from the batchRows
	rowids := make([]int32, len(pendingRows))
	for i, row := range pendingRows {
		rowids[i] = row.Rowid
	}

	// Update the batchrows status to aborted for rows with status queued or inprog
	err = jm.Queries.UpdateBatchRowsStatus(context.Background(), batchsqlc.UpdateBatchRowsStatusParams{
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
	expiry := time.Duration(jm.Config.BatchStatusCacheDurSec*100) * time.Second
	err = jm.RedisClient.Set(context.Background(), redisKey, string(batchsqlc.StatusEnumAborted), expiry).Err()
	if err != nil {
		log.Printf("failed to set Redis batch status: %v", err)
	}

	return nil
}
func (jm *JobManager) SlowQueryList(req ListInput) (sqlist []SlowQueryDetails_t, err error) {

	sqlist = make([]SlowQueryDetails_t, 0)

	// Calculate the threshold time based on the age in days
	thresholdTime := time.Now().AddDate(0, 0, -int(req.Age))

	// Create a pgtype.Timestamp with the calculated time
	timestamp := pgtype.Timestamp{Time: thresholdTime, Valid: true}

	op := batchsqlc.NullTypeEnum{Valid: false}
	if req.Op != nil {
		op = batchsqlc.NullTypeEnum{TypeEnum: batchsqlc.TypeEnum(*req.Op), Valid: true}
	}

	// Fetch the slowQueryList based on parameters
	responseData, err := jm.Queries.FetchSlowQueryList(context.Background(), batchsqlc.FetchSlowQueryListParams{
		App: req.App,
		Op:  op,
		Age: timestamp,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch slow query list: %v", err)
	}

	if len(responseData) > 0 {
		for _, row := range responseData {
			var outputfiles map[string]string
			if row.Outputfiles != nil {
				err := json.Unmarshal(row.Outputfiles, &outputfiles)
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal outputfiles: %v", err)
				}
			}

			status := getBatchStatus(row.Status)

			detail := SlowQueryDetails_t{
				Id:          row.ID.String(),
				App:         row.App,
				Op:          row.Op,
				Inputfile:   row.Inputfile.String,
				Status:      status,
				Reqat:       row.Reqat.Time,
				Doneat:      row.Doneat.Time,
				Outputfiles: outputfiles,
			}

			sqlist = append(sqlist, detail)
		}
	}

	return sqlist, nil
}
