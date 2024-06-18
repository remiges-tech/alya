CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE status_enum AS ENUM ('queued', 'inprog', 'success', 'failed', 'aborted', 'wait');
CREATE TYPE type_enum AS ENUM ('Q','B');

CREATE TABLE batches (
    id UUID NOT NULL PRIMARY KEY DEFAULT uuid_generate_v4(),
    app VARCHAR(255) NOT NULL,
    op VARCHAR(255) NOT NULL CHECK (op = LOWER(op)), 
    type type_enum NOT NULL,
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

---- create above / drop below ----

DROP TABLE IF EXISTS batchrows CASCADE;
DROP TABLE IF EXISTS batches CASCADE;
DROP TYPE status_enum;
DROP TYPE type_enum;

