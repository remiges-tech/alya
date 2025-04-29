-- Add indexes to improve performance of batch processing queries

-- For filtering batchrows by status
CREATE INDEX IF NOT EXISTS idx_batchrows_status ON batchrows(status);

-- For more efficient batch+status filtering
CREATE INDEX IF NOT EXISTS idx_batchrows_batch_status ON batchrows(batch, status);

-- For filtering batches by app
CREATE INDEX IF NOT EXISTS idx_batches_app ON batches(app);

-- For filtering batches by app+op combination
CREATE INDEX IF NOT EXISTS idx_batches_app_op ON batches(app, op);

---- create above / drop below ----

-- Drop indexes if rolling back
DROP INDEX IF EXISTS idx_batchrows_status;
DROP INDEX IF EXISTS idx_batchrows_batch_status;
DROP INDEX IF EXISTS idx_batches_app;
DROP INDEX IF EXISTS idx_batches_app_op;
