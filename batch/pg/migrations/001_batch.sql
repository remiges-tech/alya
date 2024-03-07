CREATE TYPE status_enum AS ENUM ('queued', 'inprog', 'success', 'failed', 'aborted', 'wait');

CREATE TABLE batches (
    id UUID NOT NULL PRIMARY KEY,
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
    naborted INT
);

CREATE TABLE batchrows (
    rowid SERIAL NOT NULL PRIMARY KEY,
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
    CONSTRAINT fk_batch FOREIGN KEY (batch) REFERENCES batches(id)
);
