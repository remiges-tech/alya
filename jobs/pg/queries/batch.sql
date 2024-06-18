-- name: InsertIntoBatches :one
INSERT INTO batches (id, app, op, context, status, reqat)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: InsertIntoBatchRows :exec
INSERT INTO batchrows (batch, line, input, status, reqat)
VALUES ($1, $2, $3, 'queued', $4);

-- name: BulkInsertIntoBatchRows :execrows
INSERT INTO batchrows (batch, line, input, status, reqat) 
VALUES 
    (unnest(@batch::uuid[]), unnest(@line::int[]), unnest(@input::jsonb[]), 'queued', unnest(@reqat::timestamp[]));

-- name: GetBatchStatus :one
SELECT status
FROM batches
WHERE id = $1;

-- name: FetchBatchRowsForBatchDone :many
SELECT line, status, res, messages
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
SELECT batches.app, batches.status, batches.op, batches.context, batchrows.batch, batchrows.rowid, batchrows.line, batchrows.input
FROM batchrows
INNER JOIN batches ON batchrows.batch = batches.id
WHERE batchrows.status = $1 AND batches.status != 'wait'
LIMIT $2
FOR UPDATE OF batchrows SKIP LOCKED;


-- name: UpdateBatchRowsStatus :exec
UPDATE batchrows
SET status = $1
WHERE rowid = ANY($2::int[]);

-- name: GetCompletedBatches :many
SELECT id
FROM batches
WHERE status IN ('success', 'failed', 'aborted')
FOR UPDATE;

-- name: GetBatchByID :one
SELECT id, app, op, context, inputfile, status, reqat, doneat, outputfiles, nsuccess, nfailed, naborted
FROM batches
WHERE id = $1 
FOR UPDATE;

-- name: GetPendingBatchRows :many
SELECT rowid, line, input, status, reqat, doneat, res, blobrows, messages, doneby
FROM batchrows
WHERE batch = $1 AND status IN ('queued', 'inprog')
FOR UPDATE; 

-- name: GetBatchRowsByBatchIDSorted :many
SELECT rowid, line, input, status, reqat, doneat, res, blobrows, messages, doneby
FROM batchrows
WHERE batch = $1
ORDER BY line
FOR UPDATE;

-- name: GetProcessedBatchRowsByBatchIDSorted :many
SELECT rowid, line, input, status, reqat, doneat, res, blobrows, messages, doneby
FROM batchrows
WHERE batch = $1 AND status IN ('success', 'failed')
ORDER BY line
FOR UPDATE;

-- name: CountBatchRowsByBatchIDAndStatus :one
SELECT COUNT(*)
FROM batchrows
WHERE batch = $1 AND status IN ($2, $3);

-- name: UpdateBatchSummary :exec
UPDATE batches
SET status = $2, doneat = $3, outputfiles = $4, nsuccess = $5, nfailed = $6, naborted = $7
WHERE id = $1;

-- name: UpdateBatchSummaryOnAbort :exec
UPDATE batches
SET status = $2, doneat = $3, naborted = $4
WHERE id = $1;

-- name: UpdateBatchCounters :exec
UPDATE batches
SET nsuccess = COALESCE(nsuccess, 0) + $2,
    nfailed = COALESCE(nfailed, 0) + $3,
    naborted = COALESCE(naborted, 0) + $4
WHERE id = $1;

-- name: GetBatchRowsByBatchID :many
SELECT * FROM batchrows WHERE batch = $1;

-- name: UpdateBatchStatus :exec
UPDATE batches
SET status = $2, doneat = $3, outputfiles = $4, nsuccess = $5, nfailed = $6, naborted = $7
WHERE id = $1;

-- name: GetBatchRowsCount :one
SELECT COUNT(*) FROM batchrows WHERE batch = $1;

-- name: UpdateBatchRowStatus :exec
UPDATE batchrows
SET status = $2
WHERE rowid = $1;

-- name: FetchSlowQueryList :many
SELECT id, app, op, inputfile, status, reqat, doneat, outputfiles
FROM batches
WHERE type = 'Q'
AND app = @app
AND op = (sqlc.narg('op'):: type_enum) IS NULL OR (sqlc.narg('op')::type_enum)
AND reqat >= @age;

-- name: FetchBatchList :many
SELECT id, app, op, inputfile, status, reqat, doneat, outputfiles,nsuccess, nfailed, naborted
FROM batches
WHERE type = 'B'
AND app = @app
AND op = (sqlc.narg('op'):: type_enum) IS NULL OR (sqlc.narg('op')::type_enum)
AND reqat >= @age;

-- name: GetNRowsByBatchID :one
SELECT COUNT(*) FROM batchrows WHERE batch = $1;