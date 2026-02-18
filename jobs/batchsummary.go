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

	// Try to acquire advisory lock (non-blocking)
	locked, err := q.TryAdvisoryLockBatch(ctx, batchID.String())
	if err != nil {
		jm.logger.Error(err).LogActivity("Failed to try advisory lock", map[string]any{
			"batchId": batchID.String(),
		})
		return fmt.Errorf("failed to try advisory lock: %v", err)
	}

	if !locked {
		jm.logger.Info().LogActivity("Batch summarization in progress by another worker, skipping", map[string]any{
			"batchId": batchID.String(),
		})
		return ErrBatchLockNotAcquired
	}

	jm.logger.Info().LogActivity("Acquired advisory lock for batch summarization", map[string]any{
		"batchId": batchID.String(),
	})

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

	// Check queued count first, then inprog only if queued=0.
	// Most checks occur during concurrent processing (queued > 0). Checking queued first
	// allows immediate skip without querying inprog count.
	//
	// Race condition fix: When queued=0 and inprog>0, transaction isolation may show stale
	// snapshot. Row completion commits might not be visible yet. Retry with fresh transaction
	// to get updated snapshot.
	//
	// Better solution: Use Redis atomic counter (HINCRBY) to track completion. Only the worker
	// completing the last row attempts summarization. Eliminates redundant attempts and race.
	// Alternative: PostgreSQL NOTIFY/LISTEN for event-driven summarization.

	jm.logger.Info().LogActivity("Checking for queued batch rows", map[string]any{
		"batchId": batchID.String(),
	})

	queuedCount, err := q.CountBatchRowsQueuedByBatchID(ctx, batchID)
	if err != nil {
		jm.logger.Error(err).LogActivity("Failed to count queued batch rows", map[string]any{
			"batchId": batchID.String(),
		})
		return fmt.Errorf("failed to count queued batch rows: %v", err)
	}

	if queuedCount > 0 {
		// Batch incomplete, no race possible on queued rows
		jm.logger.Info().LogActivity("Batch has queued rows, skipping summarization", map[string]any{
			"batchId":      batchID.String(),
			"queuedCount":  queuedCount,
			"willRetry":    false,
		})
		return nil
	}

	// All rows fetched, check if any still processing
	jm.logger.Info().LogActivity("No queued rows, checking inprog rows", map[string]any{
		"batchId": batchID.String(),
	})

	inprogCount, err := q.CountBatchRowsInProgByBatchID(ctx, batchID)
	if err != nil {
		jm.logger.Error(err).LogActivity("Failed to count inprog batch rows", map[string]any{
			"batchId": batchID.String(),
		})
		return fmt.Errorf("failed to count inprog batch rows: %v", err)
	}

	jm.logger.Info().LogActivity("Pending row count result", map[string]any{
		"batchId":      batchID.String(),
		"queuedCount":  queuedCount,
		"inprogCount":  inprogCount,
		"willSummarize": inprogCount == 0,
	})

	if inprogCount > 0 {
		// Rows processing or stale snapshot - retry with fresh transaction
		jm.logger.Info().LogActivity("Batch has inprog rows", map[string]any{
			"batchId":     batchID.String(),
			"inprogCount": inprogCount,
		})
		return ErrBatchHasPendingRows
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
	jm.logger.Info().LogActivity("Updating batch summary in database", map[string]any{
		"batchId": batchID.String(),
		"status": batchStatus,
		"outputFileCount": len(objStoreFiles),
		"nSuccess": nsuccess,
		"nFailed": nfailed,
		"nAborted": naborted,
	})
	err = updateBatchSummary(q, ctx, batchID, batchStatus, objStoreFiles, nsuccess, nfailed, naborted)
	if err != nil {
		jm.logger.Error(err).LogActivity("Failed to update batch summary", map[string]any{
			"batchId": batchID.String(),
		})
		return fmt.Errorf("failed to update batch summary: %v", err)
	}

	jm.logger.Info().LogActivity("Batch summary database update completed successfully", map[string]any{
		"batchId": batchID.String(),
		"status": batchStatus,
	})

	// Update status in redis
	jm.logger.Debug0().LogActivity("Updating batch status in Redis cache", map[string]any{
		"batchId":       batchID.String(),
		"status":        batchStatus,
		"cacheDuration": 100 * jm.config.BatchStatusCacheDurSec,
	})
	err = updateStatusInRedis(jm.redisClient, batchID, batchStatus, 100*jm.config.BatchStatusCacheDurSec)
	if err != nil {
		jm.logger.Warn().LogActivity("Failed to update status in Redis cache for batch", map[string]any{
			"batchId": batchID.String(),
			"error":   err.Error(),
		})
		// Continue with batch completion despite Redis failure - Redis is just a cache
	}

	// Cache the batch summary for BatchStatus API
	err = updateBatchSummaryInRedis(jm.redisClient, batchID, batchStatus, objStoreFiles,
		int(nsuccess), int(nfailed), int(naborted), 100*jm.config.BatchStatusCacheDurSec)
	if err != nil {
		jm.logger.Warn().LogActivity("Failed to cache batch summary in Redis", map[string]any{
			"batchId": batchID.String(),
			"error":   err.Error(),
		})
		// Continue despite Redis failure - Redis is just a cache
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

// updateStatusInRedis updates the batch status in Redis.
// Redis SET is atomic, so no transaction is needed for single-key updates.
func updateStatusInRedis(redisClient *redis.Client, batchID uuid.UUID, status batchsqlc.StatusEnum, expirySec int) error {
	redisKey := BatchStatusKey(batchID.String())
	expiry := time.Duration(expirySec) * time.Second

	err := redisClient.Set(context.Background(), redisKey, string(status), expiry).Err()
	if err != nil {
		return fmt.Errorf("failed to update status in Redis: %w", err)
	}
	return nil
}

// updateBatchSummaryInRedis caches the batch summary (status, outputfiles, counters) in Redis.
// Uses a single key with JSON blob for atomic read/write.
func updateBatchSummaryInRedis(redisClient *redis.Client, batchID uuid.UUID,
	status batchsqlc.StatusEnum, outputFiles map[string]string,
	nsuccess, nfailed, naborted int, expirySec int) error {

	summary := BatchSummary_t{
		Status:      string(status),
		OutputFiles: outputFiles,
		NSuccess:    nsuccess,
		NFailed:     nfailed,
		NAborted:    naborted,
	}

	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("failed to marshal batch summary: %w", err)
	}

	redisKey := BatchSummaryKey(batchID.String())
	expiry := time.Duration(expirySec) * time.Second

	err = redisClient.Set(context.Background(), redisKey, summaryJSON, expiry).Err()
	if err != nil {
		return fmt.Errorf("failed to update batch summary in Redis: %w", err)
	}
	return nil
}

// updateStatusAndOutputFilesDataInRedis updates batch status, result, and output files in Redis.
// Uses TxPipeline (MULTI/EXEC) for atomic multi-key update. All keys use hash tags for
// Redis Cluster compatibility (same slot).
func updateStatusAndOutputFilesDataInRedis(redisClient *redis.Client, batchID uuid.UUID, status batchsqlc.StatusEnum, outputFiles map[string]string, result string, expirySec int) error {
	redisKey := BatchStatusKey(batchID.String())
	redisResultKey := BatchResultKey(batchID.String())
	redisOutputFilesKey := BatchOutputFilesKey(batchID.String())
	expiry := time.Duration(expirySec) * time.Second

	outputFilesJSON, err := json.Marshal(outputFiles)
	if err != nil {
		return fmt.Errorf("failed to marshal output files: %w", err)
	}

	pipe := redisClient.TxPipeline()
	pipe.Set(context.Background(), redisKey, string(status), expiry)
	pipe.Set(context.Background(), redisResultKey, result, expiry)
	pipe.Set(context.Background(), redisOutputFilesKey, outputFilesJSON, expiry)
	_, err = pipe.Exec(context.Background())
	if err != nil {
		return fmt.Errorf("failed to update status, outputfiles and result in Redis: %w", err)
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
