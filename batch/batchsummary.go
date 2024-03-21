package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/remiges-tech/alya/batch/objstore"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
)

func (jm *JobManager) summarizeBatch(q *batchsqlc.Queries, batchID uuid.UUID) error {
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

	// fetch count of records where status = queued or inprogress
	count, err := q.CountBatchRowsByBatchIDAndStatus(ctx, batchsqlc.CountBatchRowsByBatchIDAndStatusParams{
		Batch:    batchID,
		Status:   batchsqlc.StatusEnumQueued,
		Status_2: batchsqlc.StatusEnumInprog,
	})
	if err != nil {
		return fmt.Errorf("failed to count batch rows by batch ID and status: %v", err)
	}

	if count > 0 {
		return nil
	}

	// Fetch all batchrows records for the batch, sorted by "line"
	batchRows, err := q.GetBatchRowsByBatchIDSorted(ctx, batchID)
	if err != nil {
		return fmt.Errorf("failed to get batch rows sorted: %v", err)
	}

	// Calculate the summary counters
	nsuccess, nfailed, naborted := calculateSummaryCounters(batchRows)

	// Determine the overall batch status based on the counter values
	batchStatus := determineBatchStatus(nsuccess, nfailed, naborted)

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
	outputFiles, err := moveFilesToObjectStore(tmpFiles, jm.ObjStore, "batch-output")
	if err != nil {
		return fmt.Errorf("failed to move files to object store: %v", err)
	}

	// Update the batches record with summarized information
	err = updateBatchSummary(q, ctx, batchID, batchStatus, outputFiles, nsuccess, nfailed, naborted)
	if err != nil {
		return fmt.Errorf("failed to update batch summary: %v", err)
	}

	// Update status in redis
	err = updateStatusInRedis(jm.RedisClient, batchID, batchStatus)
	if err != nil {
		return fmt.Errorf("failed to update status in redis: %v", err)
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

			for logicalFile := range blobRows {
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
	return tmpFiles, nil
}

func cleanupTemporaryFiles(tmpFiles map[string]*os.File) {
	for _, file := range tmpFiles {
		if err := file.Close(); err != nil {
			log.Printf("failed to close temporary file: %v", err)
		}
		if err := os.Remove(file.Name()); err != nil {
			log.Printf("failed to remove temporary file: %v", err)
		}
	}
}

func appendBlobRowsToFiles(batchRows []batchsqlc.GetProcessedBatchRowsByBatchIDSortedRow, tmpFiles map[string]*os.File) error {
	for _, row := range batchRows {
		var blobRows map[string]string
		if err := json.Unmarshal(row.Blobrows, &blobRows); err != nil {
			return fmt.Errorf("failed to unmarshal blobrows: %v", err)
		}

		for logicalFile, content := range blobRows {
			if _, err := tmpFiles[logicalFile].WriteString(content + "\n"); err != nil {
				log.Printf("Error writing to temporary file for logical file %s: %v", logicalFile, err)
				return fmt.Errorf("failed to write to temporary file: %v", err)
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

func updateBatchSummary(q *batchsqlc.Queries, ctx context.Context, batchID uuid.UUID, status batchsqlc.StatusEnum, outputFiles map[string]string, nsuccess, nfailed, naborted int64) error {
	outputFilesJSON, err := json.Marshal(outputFiles)
	if err != nil {
		return fmt.Errorf("failed to marshal output files: %v", err)
	}

	err = q.UpdateBatchSummary(ctx, batchsqlc.UpdateBatchSummaryParams{
		ID:          batchID,
		Status:      status,
		Doneat:      pgtype.Timestamp{Time: time.Now()},
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

func updateStatusInRedis(redisClient *redis.Client, batchID uuid.UUID, status batchsqlc.StatusEnum) error {
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", batchID)
	expiry := time.Duration(ALYA_BATCHSTATUS_CACHEDUR_SEC*100) * time.Second
	_, err := redisClient.Set(redisKey, string(status), expiry).Result()
	if err != nil {
		return fmt.Errorf("failed to update status in redis: %v", err)
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
