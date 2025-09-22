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
	tx, err := jm.db.Begin(context.Background())
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

	// Create transaction-bound queries
	txQueries := batchsqlc.New(tx)

	// Convert op to lowercase before inserting into the database
	op = strings.ToLower(op)

	// Use sqlc generated function to insert into batches table
	_, err = txQueries.InsertIntoBatches(ctx, batchsqlc.InsertIntoBatchesParams{
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
	err = txQueries.InsertIntoBatchRows(ctx, batchsqlc.InsertIntoBatchRowsParams{
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

func (jm *JobManager) SlowQueryDone(reqID string) (status BatchStatus_t, result JSONstr, messages []wscutils.ErrorMessage, outputfiles map[string]string, err error) {

	// Check REDIS for the status
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", reqID)
	redisResultKey := fmt.Sprintf("ALYA_BATCHRESULT_%s", reqID)
	redisOutputFilesKey := fmt.Sprintf("ALYA_BATCHOUTFILES_%s", reqID)

	reqIDUUID, err := uuid.Parse(reqID)
	if err != nil {
		log.Printf("SlowQuery.Done invalid request ID: %v", err)
		result, _ := NewJSONstr("")
		return BatchTryLater, result, nil, outputfiles, fmt.Errorf("invalid request ID: %v", err)
	}
	statusVal, err := jm.redisClient.Get(context.Background(), redisKey).Result()
	if err == redis.Nil {
		// Key does not exist in REDIS, check the database
		batchStatus, resultData, outputfiles, err := getBatchDetails(jm, reqIDUUID)
		if err != nil {
			log.Printf("SlowQuery.Done GetBatchDetails failed for request %v: %v", reqID, err)
			result, _ := NewJSONstr("")
			return BatchTryLater, result, nil, outputfiles, err // Assuming GetBatchDetails returns an error if not found
		}
		if batchStatus == batchsqlc.StatusEnumSuccess || batchStatus == batchsqlc.StatusEnumFailed {
			result = resultData
		}

		// Convert string to StatusEnum if necessary
		var enumStatus batchsqlc.StatusEnum
		enumStatus.Scan(batchStatus) // Assuming batchStatus is a string, adjust if it's already StatusEnum

		// Determine the BatchStatus_t based on batchStatus
		status := getBatchStatus(enumStatus)

		// Insert/update REDIS with 100x expiry if not found earlier
		expirySec := jm.config.BatchStatusCacheDurSec
		if status == BatchSuccess || status == BatchFailed || status == BatchAborted {
			expirySec = 100 * jm.config.BatchStatusCacheDurSec
		}

		if err := updateStatusAndOutputFilesDataInRedis(jm.redisClient, reqIDUUID, batchStatus, outputfiles, resultData.String(), expirySec); err != nil {
			jm.logger.Warn().LogActivity("Failed to update status and output files in Redis cache for slow query", map[string]any{
				"reqId": reqIDUUID.String(),
				"error": err.Error(),
			})
			// Continue with slow query completion despite Redis failure - Redis is just a cache
		}

		// Return the formatted result, messages, and nil for error
		return status, result, messages, outputfiles, nil

	} else if err != nil {
		result, _ := NewJSONstr("")
		return BatchTryLater, result, nil, outputfiles, err
	} else {

		// Key exists in REDIS, determine the action based on its value

		// Fetch the result and outputFiles from Redis
		resultVal, err := jm.redisClient.Get(context.Background(), redisResultKey).Result()
		if err != nil {
			result, _ := NewJSONstr("")
			return BatchTryLater, result, nil, outputfiles, err
		}

		outputFilesVal, err := jm.redisClient.Get(context.Background(), redisOutputFilesKey).Result()
		if err != nil {
			result, _ := NewJSONstr("")
			return BatchTryLater, result, nil, outputfiles, err
		}

		// Convert the values to the appropriate types
		result, err = NewJSONstr(resultVal)
		if err != nil {
			return BatchTryLater, result, nil, outputfiles, err
		}

		err = json.Unmarshal([]byte(outputFilesVal), &outputfiles)
		if err != nil {
			return BatchTryLater, result, nil, outputfiles, fmt.Errorf("failed to unmarshal output files: %v", err)
		}

		var enumStatus batchsqlc.StatusEnum
		enumStatus.Scan(statusVal) // Convert Redis result to StatusEnum
		status = getBatchStatus(enumStatus)

		return status, result, messages, outputfiles, nil

	}
}

func (jm *JobManager) SlowQueryAbort(reqID string) (err error) {
	// Parse the request ID as a UUID
	reqIDUUID, err := uuid.Parse(reqID)
	if err != nil {
		return fmt.Errorf("invalid request ID: %v", err)
	}

	// Start a transaction
	tx, err := jm.db.Begin(context.Background())
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback(context.Background())

	// Perform SELECT FOR UPDATE on batches and batchrows for the given request ID
	batch, err := jm.queries.GetBatchByID(context.Background(), reqIDUUID)
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
	err = jm.queries.UpdateBatchSummary(context.Background(), batchsqlc.UpdateBatchSummaryParams{
		ID:     reqIDUUID,
		Status: batchsqlc.StatusEnumAborted,
		Doneat: pgtype.Timestamp{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to update batch summary: %v", err)
	}

	// Fetch the pending batchrows records associated with the batch ID
	pendingRows, err := jm.queries.GetPendingBatchRows(context.Background(), reqIDUUID)
	if err != nil {
		return fmt.Errorf("failed to get pending batchrows: %v", err)
	}

	// Extract the rowids from the batchRows
	rowids := make([]int64, len(pendingRows))
	for i, row := range pendingRows {
		rowids[i] = row.Rowid
	}

	// Update the batchrows status to aborted for rows with status queued or inprog
	err = jm.queries.UpdateBatchRowsStatus(context.Background(), batchsqlc.UpdateBatchRowsStatusParams{
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
	expiry := time.Duration(jm.config.BatchStatusCacheDurSec*100) * time.Second
	err = jm.redisClient.Set(context.Background(), redisKey, string(batchsqlc.StatusEnumAborted), expiry).Err()
	if err != nil {
		log.Printf("failed to set Redis batch status: %v", err)
	}

	return nil
}

// Function to get output files, result, and status
func getBatchDetails(jm *JobManager, reqIDUUID uuid.UUID) (status batchsqlc.StatusEnum, result JSONstr, outputfiles map[string]string, err error) {
	var (
		batchStatus                 batchsqlc.StatusEnum
		outputFilesData, resultData []byte
	)

	batchData, err := jm.queries.GetBatchStatusAndOutputFiles(context.Background(), reqIDUUID)
	if err != nil {
		log.Printf("getBatchDetails GetBatchStatusAndOutputFiles failed for request %v: %v", reqIDUUID, err)
		result, _ = NewJSONstr("")
		return batchStatus, result, nil, err
	}

	// TODO: This condition is too restrictive for failed queries. When a slow query fails,
	// the res and outputfiles fields may be empty/NULL, causing this function to report
	// "No batch records found" even though the batch exists. We should check for batch
	// existence separately from checking if results are available. Consider:
	// 1. Always return the batch status if the batch exists
	// 2. Handle empty res/outputfiles gracefully for failed/aborted batches
	// 3. Only report "No batch records found" when the query actually returns no rows
	if len(batchData.Res) != 0 && len(batchData.Outputfiles) != 0 && batchData.Status != "" {
		batchStatus = batchData.Status
		outputFilesData = batchData.Outputfiles
		resultData = batchData.Res

		// Unmarshal outputFilesData into a map[string]string
		err = json.Unmarshal(outputFilesData, &outputfiles)
		if err != nil {
			log.Printf("Failed to unmarshal output files: %v", err)
			return batchStatus, result, nil, err
		}

		// Unmarshal resultData into a JSONstr
		err = json.Unmarshal(resultData, &result)
		if err != nil {
			log.Printf("Failed to unmarshal result: %v", err)
			result.valid = false
		} else {
			result.value = string(batchData.Res)
			result.valid = true
		}
	} else {
		log.Printf("No batch records found for request %v", reqIDUUID)
	}

	return batchStatus, result, outputfiles, nil
}
