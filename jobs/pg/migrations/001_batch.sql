CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE status_enum AS ENUM ('queued', 'inprog', 'success', 'failed', 'aborted', 'wait');

-- Table to store batch job information
CREATE TABLE batches (
    id UUID NOT NULL PRIMARY KEY DEFAULT uuid_generate_v4(),
    app VARCHAR(255) NOT NULL,
    op VARCHAR(255) NOT NULL CHECK (op = LOWER(op)), 
    context JSONB NOT NULL,
    inputfile VARCHAR(255),
    status status_enum NOT NULL,
    reqat TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    doneat TIMESTAMP WITHOUT TIME ZONE,
    outputfiles JSONB,
    nsuccess INT,
    nfailed INT,
    naborted INT,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW() 
);

-- Table to store individual rows of batch jobs
CREATE TABLE batchrows (
    rowid BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    batch UUID NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    line INT NOT NULL,
    input JSONB NOT NULL,
    status status_enum NOT NULL,
    reqat TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    doneat TIMESTAMP WITHOUT TIME ZONE,
    res JSONB,
    blobrows JSONB,
    messages JSONB,
    doneby VARCHAR(255),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(), -- Add created_at column
    CONSTRAINT fk_batch FOREIGN KEY (batch) REFERENCES batches(id)
);

-- Table to store information about files associated with batches
CREATE TABLE batch_files (
    id SERIAL PRIMARY KEY,
    batch_id UUID NOT NULL REFERENCES batches(id),
    object_id TEXT NOT NULL,
    filename TEXT NOT NULL,
    size BIGINT NOT NULL,
    checksum TEXT NOT NULL,
    content_type TEXT NOT NULL,
    status BOOLEAN NOT NULL,
    received_at TIMESTAMP WITH TIME ZONE NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    metadata JSONB,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT NOW(), -- Add created_at column
    CONSTRAINT unique_object_id UNIQUE (object_id)
);

-- Comments for batch_files table and its columns
COMMENT ON TABLE batch_files IS 'Stores metadata for files associated with batch jobs';
COMMENT ON COLUMN batch_files.id IS 'Unique identifier for each batch file record';
COMMENT ON COLUMN batch_files.batch_id IS 'Reference to the associated batch in the batches table';
COMMENT ON COLUMN batch_files.object_id IS 'Unique identifier for the file in the object store';
COMMENT ON COLUMN batch_files.filename IS 'Original name of the file';
COMMENT ON COLUMN batch_files.size IS 'Size of the file in bytes';
COMMENT ON COLUMN batch_files.checksum IS 'Hash or checksum of the file contents for integrity verification';
COMMENT ON COLUMN batch_files.content_type IS 'MIME type of the file';
COMMENT ON COLUMN batch_files.status IS 'Indicates whether the file was successfully processed (TRUE) or failed (FALSE)';
COMMENT ON COLUMN batch_files.received_at IS 'Timestamp when the file was received and stored in the object store';
COMMENT ON COLUMN batch_files.processed_at IS 'Timestamp when the file was processed by the batch system';
COMMENT ON COLUMN batch_files.error_message IS 'Contains any error message if the file processing failed';
COMMENT ON COLUMN batch_files.metadata IS 'Additional metadata about the file in JSONB format';

-- Index for faster lookups by batch_id in batch_files table
CREATE INDEX idx_batch_files_batch_id ON batch_files(batch_id);

-- Index for faster lookups by object_id in batch_files table
CREATE INDEX idx_batch_files_object_id ON batch_files(object_id);

---- create above / drop below ----

DROP TABLE IF EXISTS batch_files CASCADE;
DROP TABLE IF EXISTS batchrows CASCADE;
DROP TABLE IF EXISTS batches CASCADE;
DROP TYPE status_enum;

