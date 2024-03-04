package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/batch/pg/sqlc"
	"github.com/remiges-tech/alya/wscutils"
)

const ALYA_BATCHSTATUS_CACHEDUR_SEC = 100

type SlowQuery struct {
	Db          *pgxpool.Pool
	Queries     sqlc.Querier
	RedisClient *redis.Client
}

type JSONstr string

// Assuming sqlc generated the following methods in the db package:
// - InsertIntoBatches(ctx context.Context, arg db.InsertIntoBatchesParams) (db.Batch, error)
// - InsertIntoBatchRows(ctx context.Context, arg db.InsertIntoBatchRowsParams) error

func (s SlowQuery) Submit(app, op string, inputContext, input JSONstr) (reqID string, err error) {
	// Previous validation code remains unchanged

	// Generate a unique request ID
	id := uuid.New().String()

	// Start a database transaction
	tx, err := s.Db.Begin(context.Background())
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())

	ctx := context.Background()

	// Use sqlc generated function to insert into batches table
	_, err = s.Queries.InsertIntoBatches(ctx, sqlc.InsertIntoBatchesParams{
		App:     sqlc.AppEnum(app),
		Op:      op,
		Context: json.RawMessage(inputContext),
	})
	if err != nil {
		return "", err
	}

	// Use sqlc generated function to insert into batchrows table
	err = s.Queries.InsertIntoBatchRows(ctx, sqlc.InsertIntoBatchRowsParams{
		Line:  1,
		Input: json.RawMessage(input),
	})
	if err != nil {
		return "", err
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	// Return the UUID as reqID and nil for err
	return id, nil
}

type BatchStatus_t int

const (
	BatchTryLater BatchStatus_t = iota
	BatchSuccess
	BatchFailed
	BatchAborted
)

func (s SlowQuery) Done(reqID string) (status BatchStatus_t, result JSONstr, messages []wscutils.ErrorMessage, err error) {
	// Check REDIS for the status
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", reqID)
	statusVal, err := s.RedisClient.Get(redisKey).Result()
	if err == redis.Nil {
		// Key does not exist in REDIS, check the database
		reqUUID, err := uuid.Parse(reqID)
		if err != nil {
			// Handle error if the string is not a valid UUID
			return BatchTryLater, "", nil, fmt.Errorf("invalid request ID: %v", err)
		}
		batchStatus, err := s.Queries.GetBatchStatus(context.Background(), reqUUID)
		if err != nil {
			return BatchTryLater, "", nil, err // Assuming GetBatchStatus returns an error if not found
		}

		// Based on batchStatus, decide the next steps
		switch batchStatus {
		case "success", "failed", "aborted":
			var result JSONstr
			var messages []wscutils.ErrorMessage

			// Fetch data from batchrows if status is success or failed
			if batchStatus != "aborted" {
				rowsData, err := s.Queries.FetchBatchRowsData(context.Background(), reqUUID)
				if err != nil {
					return BatchTryLater, "", nil, err
				}

				// Assuming you want to convert rowsData to JSONstr
				// This requires rowsData to be serializable to JSON
				jsonData, err := json.Marshal(rowsData)
				if err != nil {
					// Handle JSON marshaling error
					return BatchTryLater, "", nil, fmt.Errorf("error marshaling rows data to JSON: %v", err)
				}
				result = JSONstr(jsonData)
			}

			// Determine the BatchStatus_t based on batchStatus
			status := determineBatchStatus(string(batchStatus))

			// Insert/update REDIS with 100x expiry if not found earlier
			expiry := 100 * ALYA_BATCHSTATUS_CACHEDUR_SEC // Assuming ALYA_BATCHSTATUS_CACHEDUR_SEC is defined globally
			s.RedisClient.Set(redisKey, batchStatus, time.Second*time.Duration(expiry))

			// You might need to set messages based on your application logic

			// Return the formatted result, messages, and nil for error
			return status, result, messages, nil

		default:
			// Insert/update REDIS with normal expiry and return BatchTryLater
			s.RedisClient.Set(redisKey, batchStatus, time.Second*time.Duration(ALYA_BATCHSTATUS_CACHEDUR_SEC))
			return BatchTryLater, "", nil, nil
		}
	} else if err != nil {
		return BatchTryLater, "", nil, err
	} else {
		// Key exists in REDIS, determine the action based on its value
		status = determineBatchStatus(statusVal)
	}

	// Format the response based on the status
	// This part is left as an exercise, assuming functions like determineBatchStatus and fetchBatchRowsData are implemented
	return status, result, messages, nil
}

// determineBatchStatus converts a batch status string from the database or Redis
// to a BatchStatus_t value.
// TODO: this may not be required if we just use proper enum with String() method
func determineBatchStatus(status string) BatchStatus_t {
	switch status {
	case "success":
		return BatchSuccess
	case "failed":
		return BatchFailed
	case "aborted":
		return BatchAborted
	default:
		// This includes "queued", "inprog", "wait", or any other unexpected value.
		// You might want to log or handle unexpected values differently.
		return BatchTryLater
	}
}
