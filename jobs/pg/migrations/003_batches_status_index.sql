CREATE INDEX IF NOT EXISTS idx_batches_status ON batches(status);

---- create above / drop below ----

DROP INDEX IF EXISTS idx_batches_status;
