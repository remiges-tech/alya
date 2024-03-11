package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
)

type Batch struct {
	Db          *pgxpool.Pool
	Queries     batchsqlc.Querier
	RedisClient *redis.Client
}

func (b Batch) Submit(app, op string, batchctx JSONstr, batchInput []batchsqlc.InsertIntoBatchRowsParams) (batchID string, err error) {
	// Generate a unique batch ID
	batchUUID, err := uuid.NewUUID()

	// Start a transaction
	tx, err := b.Db.Begin(context.Background())
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())

	// Insert a record into the batches table
	_, err = b.Queries.InsertIntoBatches(context.Background(), batchsqlc.InsertIntoBatchesParams{
		ID:      batchUUID,
		App:     app,
		Op:      op,
		Context: []byte(batchctx),
	})
	if err != nil {
		return "", err
	}

	// Insert records into the batchrows table
	// TODO: do it in bulk
	for _, input := range batchInput {
		input.Batch = batchUUID
		err := b.Queries.InsertIntoBatchRows(context.Background(), input)
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

func (b Batch) Done(batchID string) (status batchsqlc.StatusEnum, batchOutput []batchsqlc.FetchBatchRowsDataRow, outputFiles map[string]string, nsuccess, nfailed, naborted int, err error) {
	var batch batchsqlc.Batch
	// Check REDIS for the batch status
	redisKey := fmt.Sprintf("ALYA_BATCHSTATUS_%s", batchID)
	statusVal, err := b.RedisClient.Get(redisKey).Result()
	if err == redis.Nil {
		// Key does not exist in REDIS, check the database
		batch, err := b.Queries.GetBatchByID(context.Background(), uuid.MustParse(batchID))
		if err != nil {
			return batchsqlc.StatusEnumWait, nil, nil, 0, 0, 0, err
		}
		status = batch.Status

		// Update REDIS with batches.status and an expiry duration
		expiry := time.Duration(ALYA_BATCHSTATUS_CACHEDUR_SEC*100) * time.Second
		err = b.RedisClient.Set(redisKey, string(batch.Status), expiry).Err()
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
		batchOutput, err = b.Queries.FetchBatchRowsData(context.Background(), uuid.MustParse(batchID))
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
