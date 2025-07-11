package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/remiges-tech/alya/jobs/objstore"
	"github.com/remiges-tech/alya/jobs/pg/batchsqlc"
)

func (jm *JobManager) summarizeBatch(q batchsqlc.Querier, batchID uuid.UUID) error {
	ctx := context.Background()

	// Fetch the batch record
	jm.logger.Debug0().LogActivity("Fetching batch record for summarization", map[string]any{
		"batchId": batchID.String(),
	})
	batch, err := q.GetBatchByID(ctx, batchID)
	if err != nil {
		jm.logger.Error(err).LogActivity("Failed to get batch by ID", map[string]any{
			"batchId": batchID.String(),
		})
		return fmt.Errorf("failed to get batch by ID: %v", err)
	}

	// Check if the batch is already summarized
	if !batch.Doneat.Time.IsZero() {
		jm.logger.Debug0().LogActivity("Batch already summarized", map[string]any{
			"batchId": batchID.String(),
			"doneAt": batch.Doneat.Time,
		})
		return nil
	}

	// fetch count of records where status = queued or inprogress
	jm.logger.Debug0().LogActivity("Checking for pending batch rows", map[string]any{
		"batchId": batchID.String(),
	})
	count, err := q.CountBatchRowsByBatchIDAndStatus(ctx, batchsqlc.CountBatchRowsByBatchIDAndStatusParams{
		Batch:    batchID,
		Status:   batchsqlc.StatusEnumQueued,
		Status_2: batchsqlc.StatusEnumInprog,
	})
	if err != nil {
		jm.logger.Error(err).LogActivity("Failed to count batch rows by status", map[string]any{
			"batchId": batchID.String(),
		})
		return fmt.Errorf("failed to count batch rows by batch ID and status: %v", err)
	}

	if count > 0 {
		jm.logger.Info().LogActivity("Batch has pending rows, skipping summarization", map[string]any{
			"batchId": batchID.String(),
			"pendingCount": count,
		})
		return nil
	}

	// Fetch all batchrows records for the batch, sorted by "line"
	jm.logger.Debug0().LogActivity("Fetching all batch rows for summarization", map[string]any{
		"batchId": batchID.String(),
	})
	batchRows, err := q.GetBatchRowsByBatchIDSorted(ctx, batchID)
	if err != nil {
		jm.logger.Error(err).LogActivity("Failed to get batch rows sorted", map[string]any{
			"batchId": batchID.String(),
		})
		return fmt.Errorf("failed to get batch rows sorted: %v", err)
	}

	// Calculate the summary counters
	nsuccess, nfailed, naborted := calculateSummaryCounters(batchRows)
	jm.logger.Info().LogActivity("Batch summary counters calculated", map[string]any{
		"batchId": batchID.String(),
		"nSuccess": nsuccess,
		"nFailed": nfailed,
		"nAborted": naborted,
		"totalRows": len(batchRows),
	})

	// Determine the overall batch status based on the counter values
	batchStatus := determineBatchStatus(nsuccess, nfailed, naborted)
	jm.logger.Info().LogActivity("Batch status determined", map[string]any{
		"batchId": batchID.String(),
		"status": batchStatus,
	})

	// Fetch processed batchrows records for the batch to create temporary files
	processedBatchRows, err := q.GetProcessedBatchRowsByBatchIDSorted(ctx, batchID)
	if err != nil {
		return fmt.Errorf("failed to get processed batch rows sorted: %v", err)
	}

	// Create temporary files for each unique logical file in blobrows
	tmpFiles, err := createTemporaryFiles(processedBatchRows)
	if err != nil {
		return fmt.Errorf("failed to create temporary files: %v", err)
	}
	defer cleanupTemporaryFiles(tmpFiles)

	// Append blobrows strings to the appropriate temporary files
	err = appendBlobRowsToFiles(processedBatchRows, tmpFiles)
	if err != nil {
		return fmt.Errorf("failed to append blobrows to files: %v", err)
	}

	// Move temporary files to the object store and update outputfiles
	objStoreFiles, err := moveFilesToObjectStore(tmpFiles, jm.objStore, jm.config.BatchOutputBucket)
	if err != nil {
		return fmt.Errorf("failed to move files to object store: %v", err)
	}

	// Update the batches record with summarized information
	jm.logger.Info().LogActivity("Updating batch summary", map[string]any{
		"batchId": batchID.String(),
		"status": batchStatus,
		"outputFileCount": len(objStoreFiles),
	})
	err = updateBatchSummary(q, ctx, batchID, batchStatus, objStoreFiles, nsuccess, nfailed, naborted)
	if err != nil {
		jm.logger.Error(err).LogActivity("Failed to update batch summary", map[string]any{
			"batchId": batchID.String(),
		})
		return fmt.Errorf("failed to update batch summary: %v", err)
	}

	// Update status in redis
	jm.logger.Debug0().LogActivity("Updating batch status in Redis cache", map[string]any{
		"batchId": batchID.String(),
		"status": batchStatus,
		"cacheDuration": 100*jm.config.BatchStatusCacheDurSec,
	})
	err = updateStatusInRedis(jm.redisClient, batchID, batchStatus, 100*jm.config.BatchStatusCacheDurSec)
	if err != nil {
		jm.logger.Error(err).LogActivity("Failed to update status in Redis", map[string]any{
			"batchId": batchID.String(),
		})
		return fmt.Errorf("failed to update status in redis: %v", err)
	}

	context, err := NewJSONstr(string(batch.Context))
	if err != nil {
		return fmt.Errorf("failed to parse context for MarkDone: %v", err)
	}

	var batchOutputFiles map[string]string
	if batch.Outputfiles != nil {
		if err := json.Unmarshal(batch.Outputfiles, &batchOutputFiles); err != nil {
			return fmt.Errorf("failed to unmarshal output files: %v", err)
		}
	}

	details := BatchDetails_t{
		ID:          batchID.String(),
		App:         batch.App,
		Op:          batch.Op,
		Context:     context,
		Status:      batchStatus,
		OutputFiles: objStoreFiles,
		NSuccess:    int(nsuccess),
		NFailed:     int(nfailed),
		NAborted:    int(naborted),
	}

	// Get or create InitBlock
	initBlock, exists := jm.initblocks[batch.App]
	if !exists || initBlock == nil {
		if initializer, exists := jm.initfuncs[batch.App]; exists {
			initBlock, err = initializer.Init(batch.App)
			if err != nil {
				return fmt.Errorf("failed to initialize app %s for MarkDone: %v", batch.App, err)
			}
			jm.initblocks[batch.App] = initBlock
		} else {
			return fmt.Errorf("no initializer found for app %s", batch.App)
		}
	}

	// After successful batch completion and Redis update, call MarkDone
	// Get the processor for this app+op
	processor, exists := jm.batchprocessorfuncs[batch.App+batch.Op]
	if exists {
		jm.logger.Info().LogActivity("Calling MarkDone for batch", map[string]any{
			"batchId": batchID.String(),
			"app": batch.App,
			"op": batch.Op,
			"processorType": "batch",
		})
		markDoneStart := time.Now()
		if err := processor.MarkDone(initBlock, context, details); err != nil {
			jm.logger.Error(err).LogActivity("MarkDone failed for batch", map[string]any{
				"batchId": batchID.String(),
				"app": batch.App,
				"op": batch.Op,
				"elapsedMs": time.Since(markDoneStart).Milliseconds(),
			})
			// We log the error but don't return it, as the batch is already complete
		} else {
			jm.logger.Debug0().LogActivity("MarkDone completed for batch", map[string]any{
				"batchId": batchID.String(),
				"elapsedMs": time.Since(markDoneStart).Milliseconds(),
			})
		}
	} else {
		slowqueryprocessor, exists := jm.slowqueryprocessorfuncs[batch.App+batch.Op]
		if exists {
			jm.logger.Info().LogActivity("Calling MarkDone for slow query", map[string]any{
				"batchId": batchID.String(),
				"app": batch.App,
				"op": batch.Op,
				"processorType": "slowquery",
			})
			markDoneStart := time.Now()
			if err := slowqueryprocessor.MarkDone(initBlock, context, details); err != nil {
				jm.logger.Error(err).LogActivity("MarkDone failed for slow query", map[string]any{
					"batchId": batchID.String(),
					"app": batch.App,
					"op": batch.Op,
					"elapsedMs": time.Since(markDoneStart).Milliseconds(),
				})
				// We log the error but don't return it, as the batch is already complete
			} else {
				jm.logger.Debug0().LogActivity("MarkDone completed for slow query", map[string]any{
					"batchId": batchID.String(),
					"elapsedMs": time.Since(markDoneStart).Milliseconds(),
				})
			}
		} else {
			jm.logger.Warn().LogActivity("No processor found for MarkDone", map[string]any{
				"app": batch.App,
				"op": batch.Op,
				"batchId": batchID.String(),
			})
		}
	}

	return nil
}

func calculateSummaryCounters(batchRows []batchsqlc.GetBatchRowsByBatchIDSortedRow) (nsuccess, nfailed, naborted int64) {
	for _, row := range batchRows {
		switch row.Status {
		case batchsqlc.StatusEnumSuccess:
			nsuccess++
		case batchsqlc.StatusEnumFailed:
			nfailed++
		case batchsqlc.StatusEnumAborted:
			naborted++
		}
	}
	return
}

func determineBatchStatus(nsuccess, nfailed, naborted int64) batchsqlc.StatusEnum {
	if naborted > 0 {
		return batchsqlc.StatusEnumAborted
	} else if nfailed > 0 {
		return batchsqlc.StatusEnumFailed
	} else {
		return batchsqlc.StatusEnumSuccess
	}
}

func createTemporaryFiles(batchRows []batchsqlc.GetProcessedBatchRowsByBatchIDSortedRow) (map[string]*os.File, error) {
	tmpFiles := make(map[string]*os.File)
	for _, row := range batchRows {
		if len(row.Blobrows) > 0 {
			var blobRows map[string]any
			if err := json.Unmarshal(row.Blobrows, &blobRows); err != nil {
				return nil, fmt.Errorf("failed to unmarshal blobrows: %v", err)
			}

			for logicalFile, content := range blobRows {
				// Check if the content is not empty before creating the temporary file
				if content != "" {
					if _, exists := tmpFiles[logicalFile]; !exists {
						file, err := os.CreateTemp("", logicalFile)
						if err != nil {
							return nil, fmt.Errorf("failed to create temporary file: %v", err)
						}
						tmpFiles[logicalFile] = file
					}
				}
			}
		}
	}
	return tmpFiles, nil
}

func cleanupTemporaryFiles(tmpFiles map[string]*os.File) {
	for _, file := range tmpFiles {
		if err := file.Close(); err != nil {
			// Use standard log for cleanup errors as we don't have access to JobManager here
			log.Printf("failed to close temporary file: %v", err)
		}
		if err := os.Remove(file.Name()); err != nil {
			log.Printf("failed to remove temporary file: %v", err)
		}
	}
}

func appendBlobRowsToFiles(batchRows []batchsqlc.GetProcessedBatchRowsByBatchIDSortedRow, tmpFiles map[string]*os.File) error {
	for _, row := range batchRows {
		if len(row.Blobrows) == 0 {
			continue
		}

		var blobRows map[string]string
		if err := json.Unmarshal(row.Blobrows, &blobRows); err != nil {
			return fmt.Errorf("failed to unmarshal blobrows: %v", err)
		}

		for logicalFile, content := range blobRows {
			content = strings.TrimSpace(content)
			if content == "" {
				continue
			}

			if file, ok := tmpFiles[logicalFile]; ok {
				// Only write if there's content and a new line if content was successfully written
				if _, err := file.WriteString(content); err != nil {
					return fmt.Errorf("failed to write content to file: %v", err)
				}
				if _, err := file.WriteString("\n"); err != nil {
					return fmt.Errorf("failed to write newline to file: %v", err)
				}
			}
		}
	}
	return nil
}

func moveFilesToObjectStore(tmpFiles map[string]*os.File, store objstore.ObjectStore, bucket string) (map[string]string, error) {
	outputFiles := make(map[string]string)
	for logicalFile, file := range tmpFiles {
		objectID, err := moveToObjectStore(file.Name(), store, bucket)
		if err != nil {
			return nil, fmt.Errorf("failed to move file to object store: %v", err)
		}
		outputFiles[logicalFile] = objectID
	}
	return outputFiles, nil
}

func updateBatchSummary(q batchsqlc.Querier, ctx context.Context, batchID uuid.UUID, status batchsqlc.StatusEnum, outputFiles map[string]string, nsuccess, nfailed, naborted int64) error {
	outputFilesJSON, err := json.Marshal(outputFiles)
	if err != nil {
		return fmt.Errorf("failed to marshal output files: %v", err)
	}

	err = q.UpdateBatchSummary(ctx, batchsqlc.UpdateBatchSummaryParams{
		ID:          batchID,
		Status:      status,
		Doneat:      pgtype.Timestamp{Time: time.Now(), Valid: true},
		Outputfiles: outputFilesJSON,
		Nsuccess:    pgtype.Int4{Int32: int32(nsuccess), Valid: true},
		Nfailed:     pgtype.Int4{Int32: int32(nfailed), Valid: true},
		Naborted:    pgtype.Int4{Int32: int32(naborted), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to update batch summary: %v", err)
	}
	return nil
}

// updateStatusInRedis updates the batch status in Redis using a transaction because multiple jobmanager
// instances could be executed parallely and without atomic update there could be incorrect update.
//
// Explanation of the transaction:
//  1. The `Watch` function is called with the batch specific key to monitor.
//     If the key's value changes after this point and before the transaction block executes, the transaction will be aborted.
//  2. Inside the transaction block, we first fetch the current status of the batch from Redis. If fetching fails or if the key does not exist (`redis.Nil`), an error is returned.
//  3. If the current status in Redis matches the new status, there is no need to update, and we exit.
//  4. If the status is different, we proceed to update the status within a pipeline. Pipelining commands means they are queued up and executed at once
//
// This is equivalent to the following Redis commands:
// 127.0.0.1:6379> WATCH ALYA_BATCHSTATUS_batchID // Monitor the key for changes
// 127.0.0.1:6379> GET ALYA_BATCHSTATUS_batchID
// 127.0.0.1:6379> MULTI // Start a transaction
// 127.0.0.1:6379> SET ALYA_BATCHSTATUS_batchID 110
// 127.0.0.1:6379> EXEC // Execute the transaction
func updateStatusInRedis(redisClient *redis.Client, batchID uuid.UUID, status batchsqlc.StatusEnum, expirySec int) error {
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", batchID)

	expiry := time.Duration(expirySec) * time.Second

	err := redisClient.Watch(context.Background(), func(tx *redis.Tx) error {
		// Check the current status of the batch in Redis
		currentStatus, err := tx.Get(context.Background(), redisKey).Result()
		if err != nil && err != redis.Nil {
			return err
		}

		// If the current status is already the same as the new status, no need to update
		if currentStatus == string(status) {
			return nil
		}

		// Update the batch status in Redis within the transaction
		_, err = tx.TxPipelined(context.Background(), func(pipe redis.Pipeliner) error {
			pipe.Set(context.Background(), redisKey, string(status), expiry)
			return nil
		})
		return err
	}, redisKey)

	if err != nil {
		return fmt.Errorf("failed to update status in Redis: %v", err)
	}
	return nil
}

func updateStatusAndOutputFilesDataInRedis(redisClient *redis.Client, batchID uuid.UUID, status batchsqlc.StatusEnum, outputFiles map[string]string, result string, expirySec int) error {
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", batchID)
	redisResultKey := fmt.Sprintf("ALYA_BATCHRESULT_%s", batchID)
	redisOutputFilesKey := fmt.Sprintf("ALYA_BATCHOUTFILES_%s", batchID)
	expiry := time.Duration(expirySec) * time.Second

	err := redisClient.Watch(context.Background(), func(tx *redis.Tx) error {
		// Check the current status of the batch in Redis
		currentStatus, err := tx.Get(context.Background(), redisKey).Result()
		if err != nil && err != redis.Nil {
			return err
		}

		// If the current status is already the same as the new status, no need to update
		if currentStatus == string(status) {
			return nil
		}
		// Convert outputFiles to JSON
		outputFilesJSON, err := json.Marshal(outputFiles)
		if err != nil {
			return fmt.Errorf("failed to marshal output files: %v", err)
		}

		// Update the batch status ,outputfiles and result in Redis within the transaction
		_, err = tx.TxPipelined(context.Background(), func(pipe redis.Pipeliner) error {
			pipe.Set(context.Background(), redisKey, string(status), expiry)
			pipe.Set(context.Background(), redisResultKey, result, expiry)
			pipe.Set(context.Background(), redisOutputFilesKey, outputFilesJSON, expiry)
			return nil
		})
		return err
	}, redisKey)

	if err != nil {
		return fmt.Errorf("failed to update status,outputfiles and result in Redis: %v", err)
	}
	return nil
}

func moveToObjectStore(filePath string, store objstore.ObjectStore, bucket string) (string, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Get the file info
	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %v", err)
	}

	// Generate a unique object name
	objectName := uuid.New().String()

	// Put the object in the object store
	err = store.Put(context.Background(), bucket, objectName, file, fileInfo.Size(), "application/octet-stream")
	if err != nil {
		return "", fmt.Errorf("failed to put object in store: %v", err)
	}

	return objectName, nil
}
