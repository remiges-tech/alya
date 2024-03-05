-- name: InsertIntoBatches :one
INSERT INTO batches (id, app, op, context, status, reqat)
VALUES ($1, $2, $3, $4, 'queued', NOW())
RETURNING id;

-- name: InsertIntoBatchRows :exec
INSERT INTO batchrows (batch, line, input, status, reqat)
VALUES ($1, $2, $3, 'queued', NOW());

-- name: GetBatchStatus :one
SELECT status
FROM batches
WHERE id = $1;

-- name: FetchBatchRowsData :many
SELECT rowid, line, input, status, reqat, doneat, res, blobrows, messages, doneby
FROM batchrows
WHERE batch = $1;

-- name: UpdateBatchRowsSlowQuery :exec
UPDATE batchrows
SET status = $2, doneat = $3, res = $4, messages = $5, doneby = $6
WHERE rowid = $1;

-- name: UpdateBatchOutputFiles :exec
UPDATE batches
SET outputfiles = $2
WHERE id = $1;

-- name: UpdateBatchRowsBatchJob :exec
UPDATE batchrows
SET status = $2, doneat = $3, res = $4, blobrows = $5, messages = $6, doneby = $7
WHERE rowid = $1;


-- name: FetchBlockOfRows :many
SELECT batches.app, batches.op, batchrows.batch, batchrows.rowid, batchrows.line, batchrows.input
FROM batchrows
INNER JOIN batches ON batchrows.batch = batches.id
WHERE batchrows.status = $1
LIMIT $2;


-- name: UpdateBatchRowsStatus :exec
UPDATE batchrows
SET status = $1
WHERE rowid = ANY($2::int[]);