CREATE TABLE tasks (
                       id SERIAL PRIMARY KEY,
                       type VARCHAR(255) NOT NULL,
                       status VARCHAR(255) NOT NULL,
                       submitted_at TIMESTAMP NOT NULL,
                       started_at TIMESTAMP DEFAULT NULL,
                       completed_at TIMESTAMP DEFAULT NULL,
                       result_path VARCHAR(255) DEFAULT NULL,
                       details JSONB
);