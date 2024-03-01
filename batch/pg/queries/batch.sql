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