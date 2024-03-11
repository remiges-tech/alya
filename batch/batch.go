package batch

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/remiges-tech/alya/batch/pg/batchsqlc"
)

type Batch struct {
	Db      *pgxpool.Pool
	Queries batchsqlc.Querier
}

func (b Batch) Submit(app, op string, batchctx JSONstr, batchInput []batchsqlc.InsertIntoBatchRowsParams) (batchID string, err error) {
	// Generate a unique batch ID
	batchID = uuid.New().String()

	// Start a transaction
	tx, err := b.Db.Begin(context.Background())
	if err != nil {
		return "", err
	}
	defer tx.Rollback(context.Background())

	// Insert a record into the batches table
	_, err = b.Queries.InsertIntoBatches(context.Background(), batchsqlc.InsertIntoBatchesParams{
		ID:      uuid.MustParse(batchID),
		App:     app,
		Op:      op,
		Context: []byte(batchctx),
	})
	if err != nil {
		return "", err
	}

	// Insert records into the batchrows table
	for _, input := range batchInput {
		input.Batch = uuid.MustParse(batchID)
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

	return batchID, nil
}

func (b Batch) Done(batchID string) (status batchsqlc.StatusEnum, batchOutput []batchsqlc.FetchBatchRowsDataRow, outputFiles map[string]string, err error) {
	// Fetch the batch record from the database
	batch, err := b.Queries.GetBatchByID(context.Background(), uuid.MustParse(batchID))
	if err != nil {
		return batchsqlc.StatusEnumQueued, nil, nil, err
	}

	status = batch.Status

	// Handle the cases based on the batch status
	switch status {
	case batchsqlc.StatusEnumSuccess, batchsqlc.StatusEnumFailed, batchsqlc.StatusEnumAborted:
		// Fetch batch rows data
		batchOutput, err = b.Queries.FetchBatchRowsData(context.Background(), uuid.MustParse(batchID))
		if err != nil {
			return status, nil, nil, err
		}

		// Fetch output files from the batches table
		outputFiles = make(map[string]string)
		json.Unmarshal(batch.Outputfiles, &outputFiles)

	case batchsqlc.StatusEnumQueued, batchsqlc.StatusEnumInprog, batchsqlc.StatusEnumWait:
		// Return with status indicating to try later
		return status, nil, nil, nil
	}

	return status, batchOutput, outputFiles, nil
}
